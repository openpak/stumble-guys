package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Matchmaking has two halves. The API signs a ticket (/matchmaking/create/signature/latest):
// MatchMakingCreationResponse {Parameters, Signature}. The client posts that JSON unchanged to the
// matchmaker (POST /request) and polls GET /request/{RequestId} until the result is Matched, then
// joins the Photon room the result names. Photon Quantum runs the round; nothing here simulates.

// mmParams is MatchMakingParameters, as signed by the API and echoed back to the matchmaker.
type mmParams struct {
	RequestId                    string
	UserId                       int
	UserName                     string
	UserSkin                     string
	UserMMR                      int
	SecureMainMmr                string
	Region                       string
	Platform                     string
	Version                      string
	UserVersion                  string
	GameMode                     string
	Variant                      string
	Suffix                       string
	IsCrossPlatform              bool
	AllowInsufficientPlayerCount bool
	MaxPlayers                   int
	IncludedLocally              int
	InclusionList                map[string]any
}

// MatchMakingResultStatus.
const (
	mmError      = 0
	mmInProgress = 1
	mmMatched    = 2
	mmNotFound   = 3
	mmExpired    = 4
)

// fillWait is how long a lobby waits for more players before it starts with the ones it has
// (the game fills the rest with bots).
var fillWait = 15 * time.Second

type mmRequest struct {
	p       mmParams
	created time.Time
	lobby   *lobby
}

type lobby struct {
	key     string
	room    string
	opened  time.Time
	max     int
	members []*mmRequest
	closed  bool
}

type matchmaker struct {
	mu       sync.Mutex
	requests map[string]*mmRequest
	open     map[string]*lobby
}

func newMatchmaker() *matchmaker {
	return &matchmaker{requests: map[string]*mmRequest{}, open: map[string]*lobby{}}
}

// matchmakingTicket answers the API's ticket call with signed parameters for this player.
func (s *server) matchmakingTicket(w http.ResponseWriter, r *http.Request, body []byte) {
	logRequest("api", r, body)
	var req struct {
		Region, GameMode, Variant string
		IncludedLocally           int
		StumblePartyParameters    *struct{ MaxPlayers int }
	}
	json.Unmarshal(body, &req)
	a := s.users.byToken(requestIdentity(r).Session)
	p := mmParams{
		RequestId:       randHex(16),
		Region:          req.Region,
		Platform:        "Switch",
		Version:         r.URL.Query().Get("version"),
		GameMode:        req.GameMode,
		Variant:         req.Variant,
		IsCrossPlatform: true,
		// The client asks to start short of a full lobby; bots take the empty slots.
		AllowInsufficientPlayerCount: true,
		MaxPlayers:                   32,
		IncludedLocally:              req.IncludedLocally,
		InclusionList:                map[string]any{},
	}
	if req.StumblePartyParameters != nil && req.StumblePartyParameters.MaxPlayers > 0 {
		p.MaxPlayers = req.StumblePartyParameters.MaxPlayers
	}
	if a != nil {
		p.UserId, p.UserName = a.ID, a.Username
	}
	pj, _ := json.Marshal(p)
	writeJSON(w, http.StatusOK, map[string]any{"Parameters": p, "Signature": s.signJWT(map[string]any{"p": string(pj)})})
}

// matchmaking serves matchmaking.stumbleguys.com.
func (s *server) matchmaking(w http.ResponseWriter, r *http.Request, path string, body []byte) {
	logRequest("mm", r, body)
	switch {
	case r.Method == http.MethodPost && strings.TrimSuffix(path, "/") == "/request":
		var t struct {
			Parameters mmParams
			Signature  string
		}
		if json.Unmarshal(body, &t) != nil || t.Parameters.RequestId == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"Status": mmError})
			return
		}
		writeJSON(w, http.StatusOK, s.mm.add(t.Parameters, s))
	case strings.HasPrefix(path, "/request/") && strings.HasSuffix(path, "/cancel"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/request/"), "/cancel")
		s.mm.cancel(id)
		writeJSON(w, http.StatusOK, map[string]any{"Success": true, "Error": ""})
	case strings.HasPrefix(path, "/request/"):
		writeJSON(w, http.StatusOK, s.mm.status(strings.TrimPrefix(path, "/request/"), s))
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"Status": mmNotFound})
	}
}

func (m *matchmaker) add(p mmParams, s *server) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	if q, ok := m.requests[p.RequestId]; ok {
		return m.resultLocked(q, s)
	}
	key := strings.Join([]string{p.GameMode, p.Variant, p.Region, p.Version}, "|")
	l := m.open[key]
	if l == nil || l.closed {
		l = &lobby{key: key, room: "OP_" + randHex(8), opened: time.Now(), max: p.MaxPlayers}
		m.open[key] = l
		log.Printf("[stumble] mm new lobby %s for %s", l.room, key)
	}
	q := &mmRequest{p: p, created: time.Now(), lobby: l}
	l.members = append(l.members, q)
	m.requests[p.RequestId] = q
	if len(l.members) >= l.max {
		l.closed = true
	}
	return m.resultLocked(q, s)
}

func (m *matchmaker) status(id string, s *server) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.requests[id]
	if !ok {
		return map[string]any{"Status": mmNotFound}
	}
	return m.resultLocked(q, s)
}

func (m *matchmaker) cancel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.requests[id]
	if !ok {
		return
	}
	delete(m.requests, id)
	if l := q.lobby; !l.closed {
		for i, x := range l.members {
			if x == q {
				l.members = append(l.members[:i], l.members[i+1:]...)
				break
			}
		}
	}
}

func (m *matchmaker) resultLocked(q *mmRequest, s *server) map[string]any {
	l := q.lobby
	if !l.closed && time.Since(l.opened) >= fillWait {
		l.closed = true
		log.Printf("[stumble] mm lobby %s starts with %d player(s)", l.room, len(l.members))
	}
	if !l.closed {
		return map[string]any{"Status": mmInProgress, "Room": "", "Users": []any{}, "Ticket": "", "Region": q.p.Region}
	}
	users := make([]map[string]any, 0, len(l.members))
	for _, x := range l.members {
		users = append(users, map[string]any{"UserId": x.p.UserId, "UserName": x.p.UserName, "UserSkin": x.p.UserSkin, "SuitableCandidatesCount": len(l.members)})
	}
	return map[string]any{
		"Status":           mmMatched,
		"Room":             l.room,
		"Users":            users,
		"Ticket":           s.signJWT(map[string]any{"room": l.room, "uid": q.p.UserId, "exp": time.Now().Add(10 * time.Minute).Unix()}),
		"ClientAppVersion": q.p.Version,
		"Region":           q.p.Region,
	}
}
