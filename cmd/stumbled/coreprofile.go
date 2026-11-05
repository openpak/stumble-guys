package main

// Who a player is comes from the account core (prds#131): this service keeps no name or country.
// A Switch credential (the BAAS user id in the NSA id token's sub) must be linked to an OpenPak
// account at first use; the account's display name and country are read from the core, cached
// for five minutes and dropped at once when the core announces profile_updated (or an unlink or
// deletion) on its event stream.

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	accountv1 "github.com/openpak/account-api/openpak/account/v1"
)

// errNotLinked: no active OpenPak account owns this Switch credential.
var errNotLinked = errors.New("no OpenPak account is linked to this console")

const profileTTL = 5 * time.Minute

type profile struct{ AccountID, Name, Country string }

// profileSource reads a linked player's profile from the core.
type profileSource interface {
	Profile(ctx context.Context, baas string) (profile, error)
}

type grpcProfiles struct {
	links  accountv1.LinksClient
	ident  accountv1.IdentityClient
	events accountv1.EventsClient
	key    string
}

func (g *grpcProfiles) ctx(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-internal-key", g.key)
}

func (g *grpcProfiles) Profile(ctx context.Context, baas string) (profile, error) {
	ctx, cancel := context.WithTimeout(g.ctx(ctx), 5*time.Second)
	defer cancel()
	l, err := g.links.GetLinkBySubject(ctx, &accountv1.GetLinkBySubjectRequest{Namespace: "switch", SubjectId: strings.ToLower(baas)})
	if status.Code(err) == codes.NotFound {
		return profile{}, errNotLinked
	}
	if err != nil {
		return profile{}, err
	}
	if l.GetState() != accountv1.LinkState_LINK_STATE_ACTIVE || l.GetAccountId() == "" {
		return profile{}, errNotLinked
	}
	a, err := g.ident.GetAccount(ctx, &accountv1.GetAccountRequest{AccountId: l.GetAccountId()})
	if err != nil {
		return profile{}, err
	}
	return profile{AccountID: l.GetAccountId(), Name: a.GetDisplayName(), Country: a.GetCountry()}, nil
}

// follow drops cache entries when the core pushes a change. It starts at the head of the stream:
// older events cannot concern entries that were read after them.
func (g *grpcProfiles) follow(ctx context.Context, c *profileCache) {
	for ctx.Err() == nil {
		err := g.followOnce(ctx, c)
		if ctx.Err() != nil {
			return
		}
		log.Printf("[stumble] account core stream: %v; resuming in 5s", err)
		c.clear() // events may have been missed
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func (g *grpcProfiles) followOnce(ctx context.Context, c *profileCache) error {
	head, err := g.events.PollEvents(g.ctx(ctx), &accountv1.PollEventsRequest{SinceVersion: 1 << 62, Limit: 1})
	if err != nil {
		return err
	}
	stream, err := g.events.SubscribeAccountEvents(g.ctx(ctx), &accountv1.SubscribeAccountEventsRequest{SinceVersion: head.GetMaxVersion()})
	if err != nil {
		return err
	}
	for {
		ev, err := stream.Recv()
		if err != nil {
			return err
		}
		switch ev.GetType() {
		case "profile_updated", "link_unlinked", "account_deleted", "account_banned":
			c.dropAccount(ev.GetAccountId())
		}
	}
}

func newProfilesFromEnv() (*profileCache, *grpcProfiles) {
	addr, key := os.Getenv("ACCOUNT_GRPC_ADDR"), os.Getenv("ACCOUNT_INTERNAL_KEY")
	if addr == "" || key == "" {
		return nil, nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("ERROR account core: %v", err)
		return nil, nil
	}
	g := &grpcProfiles{accountv1.NewLinksClient(conn), accountv1.NewIdentityClient(conn), accountv1.NewEventsClient(conn), key}
	return newProfileCache(g, profileTTL), g
}

type profileCache struct {
	src profileSource
	ttl time.Duration
	now func() time.Time
	mu  sync.Mutex
	m   map[string]cachedProfile
}

type cachedProfile struct {
	p   profile
	exp time.Time
}

func newProfileCache(src profileSource, ttl time.Duration) *profileCache {
	return &profileCache{src: src, ttl: ttl, now: time.Now, m: map[string]cachedProfile{}}
}

// get returns the linked player's profile; errNotLinked is never cached (a player who links
// right after the refusal must get in on the next try).
func (c *profileCache) get(ctx context.Context, baas string) (profile, error) {
	c.mu.Lock()
	if e, ok := c.m[baas]; ok && c.now().Before(e.exp) {
		c.mu.Unlock()
		return e.p, nil
	}
	c.mu.Unlock()
	p, err := c.src.Profile(ctx, baas)
	if err != nil {
		return profile{}, err
	}
	c.mu.Lock()
	if len(c.m) > 20000 {
		c.m = map[string]cachedProfile{}
	}
	c.m[baas] = cachedProfile{p, c.now().Add(c.ttl)}
	c.mu.Unlock()
	return p, nil
}

func (c *profileCache) dropAccount(account string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.m {
		if e.p.AccountID == account {
			delete(c.m, k)
		}
	}
}

func (c *profileCache) clear() {
	c.mu.Lock()
	c.m = map[string]cachedProfile{}
	c.mu.Unlock()
}

// fill sets the account's in-memory name and country from the core. With no core configured
// (a development rig) the old placeholders stand; a core outage keeps them too, for one response.
func (s *server) fill(ctx context.Context, a *account) {
	a.Username, a.Country = "Stumbler"+itoa(a.ID), "US"
	if s.profiles == nil || strings.HasPrefix(a.Key, "device:") {
		return
	}
	p, err := s.profiles.get(ctx, a.Key)
	if err != nil {
		return
	}
	if p.Name != "" {
		a.Username = p.Name
	}
	if len(p.Country) == 2 {
		a.Country = strings.ToUpper(p.Country)
	}
}
