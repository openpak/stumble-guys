package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// account is what the service keeps per player. Everything else in the client's User object is
// derived (owned-everything inventory) or defaulted.
type account struct {
	ID        int    `json:"id"`
	Key       string `json:"key"` // NSA id, or "device:<id>" when the console sent no NSA token
	StumbleID string `json:"stumbleId"`
	Username  string `json:"username"`
	Token     string `json:"token"`
	Created   string `json:"created"`
	Country   string `json:"country"`
	Region    string `json:"region"`
	// Equipped is the client's UserEquippedCosmeticsDto, stored as sent.
	Equipped json.RawMessage `json:"equipped,omitempty"`
	Crowns   int             `json:"crowns"`
	XP       int             `json:"xp"`
}

type store struct {
	mu     sync.Mutex
	path   string
	NextID int                 `json:"nextId"`
	ByKey  map[string]*account `json:"accounts"`
	byTok  map[string]*account
}

func openStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &store{path: filepath.Join(dir, "users.json"), NextID: 1000, ByKey: map[string]*account{}}
	if b, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(b, s); err != nil {
			return nil, err
		}
	}
	s.byTok = map[string]*account{}
	for _, a := range s.ByKey {
		s.byTok[a.Token] = a
	}
	return s, nil
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// get returns the account for key, creating it on first sight.
func (s *store) get(key string) *account {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.ByKey[key]; ok {
		return a
	}
	s.NextID++
	a := &account{
		ID:        s.NextID,
		Key:       key,
		StumbleID: randHex(12),
		Username:  "Stumbler" + itoa(s.NextID),
		Token:     randHex(24),
		Created:   nowISO(),
		Country:   "US",
		Region:    "eu",
	}
	s.ByKey[key] = a
	s.byTok[a.Token] = a
	s.saveLocked()
	return a
}

func (s *store) byToken(tok string) *account {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byTok[tok]
}

func (s *store) update(a *account, f func(*account)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(a)
	s.saveLocked()
}

func (s *store) saveLocked() {
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, s.path)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
