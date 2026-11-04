package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	acctA = "11111111-1111-4111-8111-111111111111"
	acctB = "22222222-2222-4222-8222-222222222222"
)

type fakeLinks map[string][]string

func (f fakeLinks) SwitchSubjects(_ context.Context, a string) ([]string, error) { return f[a], nil }

// Against a real Postgres (STUMBLE_TEST_DATABASE_URL).
func TestEraseRemovesOnlyThatAccount(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	st, err := openStore(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, b, dev := st.get("baasA"), st.get("baasB"), st.get("device:x")
	st.keepCollector("api.collector.scopely.io", []byte(`{"user":"`+a.StumbleID+`"}`))
	st.keepCollector("api.collector.scopely.io", []byte(`{"user":"`+b.StumbleID+`"}`))
	s := &server{users: st, mm: newMatchmaker(), links: fakeLinks{acctA: {"baasA"}, acctB: {"baasB"}}}
	s.mm.tickets["sa"] = mmParams{UserId: a.ID}
	s.mm.tickets["sb"] = mmParams{UserId: b.ID}

	n, err := s.erase(ctx, acctA)
	if err != nil || n != 2 {
		t.Fatalf("erase: %d %v", n, err)
	}
	if st.byToken(a.Token) != nil {
		t.Fatal("account kept")
	}
	if st.byToken(b.Token) == nil || st.byToken(dev.Token) == nil {
		t.Fatal("other account erased")
	}
	var c int
	st.pool.QueryRow(ctx, `SELECT count(*) FROM stumble_collector`).Scan(&c)
	if c != 1 {
		t.Fatalf("collector rows %d", c)
	}
	if _, ok := s.mm.tickets["sa"]; ok || len(s.mm.tickets) != 1 {
		t.Fatalf("tickets: %v", s.mm.tickets)
	}
	if n, err := s.erase(ctx, acctA); err != nil || n != 0 {
		t.Fatalf("second erase: %d %v", n, err)
	}
}

func TestEraseNeedsCore(t *testing.T) {
	if _, err := (&server{}).erase(context.Background(), acctA); err == nil {
		t.Fatal("erase without a core must fail")
	}
}

func TestEraseEndpoint(t *testing.T) {
	h := eraseHandler("stumble-guys", func(context.Context, string) (int, error) { return 2, nil })
	call := func(id, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("DELETE", "/internal/accounts/"+id+"/data", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "")
	if r := call(acctA, "Bearer k"); r.Code != 503 {
		t.Fatalf("unset key: %d", r.Code)
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "k")
	if r := call(acctA, ""); r.Code != 401 {
		t.Fatalf("no key: %d", r.Code)
	}
	if r := call(acctA, "Bearer bad"); r.Code != 401 {
		t.Fatalf("wrong key: %d", r.Code)
	}
	if r := call("nope", "Bearer k"); r.Code != 400 {
		t.Fatalf("bad id: %d", r.Code)
	}
	if r := call(acctA, "Bearer k"); r.Code != 200 || !strings.Contains(r.Body.String(), `"service":"stumble-guys","rows":2`) {
		t.Fatalf("ok: %d %s", r.Code, r.Body.String())
	}
}
