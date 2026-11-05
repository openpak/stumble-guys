package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeProfiles struct {
	calls int
	byKey map[string]profile
}

func (f *fakeProfiles) Profile(_ context.Context, baas string) (profile, error) {
	f.calls++
	if p, ok := f.byKey[baas]; ok {
		return p, nil
	}
	return profile{}, errNotLinked
}

func TestProfileCacheTTLAndPush(t *testing.T) {
	f := &fakeProfiles{byKey: map[string]profile{"b1": {AccountID: "a1", Name: "Ann", Country: "IE"}}}
	c := newProfileCache(f, 5*time.Minute)
	now := time.Now()
	c.now = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if p, err := c.get(ctx, "b1"); err != nil || p.Name != "Ann" {
			t.Fatalf("get: %v %v", p, err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("cached: %d calls", f.calls)
	}
	f.byKey["b1"] = profile{AccountID: "a1", Name: "Anna", Country: "IE"}
	c.dropAccount("a1") // profile_updated
	if p, _ := c.get(ctx, "b1"); p.Name != "Anna" || f.calls != 2 {
		t.Fatalf("push refresh: %v %d", p, f.calls)
	}
	now = now.Add(6 * time.Minute) // TTL
	c.get(ctx, "b1")
	if f.calls != 3 {
		t.Fatalf("ttl: %d calls", f.calls)
	}
}

func TestUnlinkedIsRefusedAndNotCached(t *testing.T) {
	f := &fakeProfiles{byKey: map[string]profile{}}
	c := newProfileCache(f, time.Minute)
	if _, err := c.get(context.Background(), "nobody"); !errors.Is(err, errNotLinked) {
		t.Fatalf("want errNotLinked, got %v", err)
	}
	f.byKey["nobody"] = profile{AccountID: "a2", Name: "N"}
	if _, err := c.get(context.Background(), "nobody"); err != nil {
		t.Fatalf("linked right after the refusal must get in: %v", err)
	}
}

func TestFillUsesCoreNameAndCountry(t *testing.T) {
	f := &fakeProfiles{byKey: map[string]profile{"b1": {AccountID: "a1", Name: "Ann", Country: "ie"}}}
	s := &server{profiles: newProfileCache(f, time.Minute)}
	a := &account{ID: 1001, Key: "b1"}
	s.fill(context.Background(), a)
	if a.Username != "Ann" || a.Country != "IE" {
		t.Fatalf("got %q %q", a.Username, a.Country)
	}
	d := &account{ID: 1002, Key: "device:x"}
	s.fill(context.Background(), d)
	if d.Username != "Stumbler1002" || d.Country != "US" {
		t.Fatalf("device fallback: %q %q", d.Username, d.Country)
	}
}
