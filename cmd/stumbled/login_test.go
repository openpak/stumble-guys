package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// The client keeps the user only from a response that starts with {"User": (Backend.HandleResponse).
func TestLoginResponseStartsWithUser(t *testing.T) {
	s, err := newServer(t.TempDir(), "", []byte("k"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/user/login", strings.NewReader(`{"DeviceId":"d","Version":"0.82"}`))
	req.Host = "api.stumbleguys.com"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), `{"User":`) {
		t.Fatalf("code %d body %.80s", rec.Code, rec.Body.String())
	}
	for _, k := range []string{`"BattlePass":{`, `"PhotonJwt":"ey`, `"StumbleId":"`} {
		if !strings.Contains(rec.Body.String(), k) {
			t.Errorf("missing %s", k)
		}
	}
}
