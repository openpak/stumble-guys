package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed shared.json
var builtinShared []byte

type server struct {
	users     *store
	shared    json.RawMessage
	sharedTag string
	key       []byte
	mm        *matchmaker
}

func keyFromEnv(flagValue string) []byte {
	if flagValue != "" {
		return []byte(flagValue)
	}
	if v := os.Getenv("STUMBLE_JWT_KEY"); v != "" {
		return []byte(v)
	}
	log.Printf("[stumble] no -jwt-key / STUMBLE_JWT_KEY: tokens are signed with a development key")
	return []byte("openpak-stumble-dev")
}

func newServer(dataDir, sharedFile string, key []byte) (*server, error) {
	st, err := openStore(dataDir)
	if err != nil {
		return nil, err
	}
	shared := builtinShared
	if sharedFile != "" {
		if shared, err = os.ReadFile(sharedFile); err != nil {
			return nil, err
		}
	}
	if !json.Valid(shared) {
		return nil, fmt.Errorf("game config is not valid JSON")
	}
	sum := sha256.Sum256(shared)
	tag := `"` + hex.EncodeToString(sum[:8]) + `"`
	return &server{users: st, shared: shared, sharedTag: tag, key: key, mm: newMatchmaker()}, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	host := strings.ToLower(r.Host)
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	path := r.URL.Path
	id := requestIdentity(r)
	log.Printf("[stumble] <- %s %s%s session=%t nsa=%t len=%d", r.Method, host, r.URL.RequestURI(), id.Session != "", id.NSAID != "", len(body))
	switch {
	case strings.HasPrefix(host, "matchmaking."):
		s.matchmaking(w, r, path, body)
		return
	case strings.HasPrefix(host, "localization."):
		logRequest("loc", r, body)
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	switch path {
	case "/user/login":
		s.login(w, r, body)
	case "/user/config":
		// The client caches the config under its ETag and sends If-none-match next time.
		logRequest("api", r, nil)
		w.Header().Set("ETag", s.sharedTag)
		if r.Header.Get("If-None-Match") == s.sharedTag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(s.shared)
	case "/onlinecheck":
		writeJSON(w, http.StatusOK, map[string]any{"Online": true})
	case "/feature-flags":
		writeJSON(w, http.StatusOK, []any{})
	case "/bots":
		// A list of BotDefinition; an empty list makes the client add its own built-in bots.
		writeJSON(w, http.StatusOK, []any{})
	case "/user-equipped-cosmetics/update":
		s.equip(w, r, body)
	case "/matchmaking/create/signature/latest", "/matchmaking/tournamentX/signature":
		s.matchmakingTicket(w, r, body)
	default:
		logRequest("api ?", r, body)
		writeJSON(w, http.StatusOK, map[string]any{})
	}
}

// readBody returns the request body, inflating it when the client compressed it.
func readBody(r *http.Request) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") || (len(b) > 2 && b[0] == 0x1f && b[1] == 0x8b) {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(io.LimitReader(zr, 32<<20))
	}
	return b, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// logRequest prints one request with its body, cut short; credentials are never printed.
func logRequest(tag string, r *http.Request, body []byte) {
	b := string(body)
	if len(b) > 600 {
		b = b[:600] + "..."
	}
	log.Printf("[stumble] %s %s %s%s body=%q", tag, r.Method, r.Host, r.URL.RequestURI(), b)
}

// identity is who a request comes from: the console's NSA id token (x-authorization) names the
// Nintendo account; the "authorization" JSON header carries our session token after login.
type identity struct {
	NSAID   string // the token's sub claim
	Session string
}

func requestIdentity(r *http.Request) identity {
	var id identity
	if t := r.Header.Get("x-authorization"); t != "" {
		id.NSAID = jwtClaim(t, "sub")
	}
	if a := r.Header.Get("authorization"); strings.HasPrefix(strings.TrimSpace(a), "{") {
		var h struct{ Token string }
		if json.Unmarshal([]byte(a), &h) == nil {
			id.Session = h.Token
		}
	}
	return id
}

// jwtClaim reads one string claim from a JWT without checking its signature (the edge already
// checks that the console holds an OpenPak device certificate).
func jwtClaim(tok, claim string) string {
	tok = strings.TrimPrefix(strings.TrimSpace(tok), "Bearer ")
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	v, _ := m[claim].(string)
	return v
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }
