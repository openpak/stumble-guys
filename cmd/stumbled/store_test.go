package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRequiresDatabase(t *testing.T) {
	if _, err := openStore(context.Background(), "", ""); err == nil {
		t.Fatal("opened without a database URL")
	}
}

// Write state, open a new instance on the same database, see it.
func TestStoreSurvivesRestart(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	s1, err := openStore(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	a := s1.get("nsa:abc")
	s1.update(a, func(a *account) { a.Crowns = 7; a.XP = 99; a.Equipped = []byte(`{"Skin":3}`) })
	s1.keepCollector("api.collector.scopely.io", []byte(`{"e":1}`))
	s1.Close()

	s2, err := openStore(ctx, dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	b := s2.byToken(a.Token)
	if b == nil || b.ID != a.ID || b.Crowns != 7 || b.XP != 99 || string(b.Equipped) != `{"Skin": 3}` && string(b.Equipped) != `{"Skin":3}` {
		t.Fatalf("lost after restart: %+v", b)
	}
	if again := s2.get("nsa:abc"); again.ID != a.ID || again.Token != a.Token {
		t.Fatal("same key got a different account")
	}
	if c := s2.get("nsa:other"); c.ID != a.ID+1 {
		t.Fatalf("id sequence: %d after %d", c.ID, a.ID)
	}
	var n int
	s2.pool.QueryRow(ctx, `SELECT count(*) FROM stumble_collector`).Scan(&n)
	if n != 1 {
		t.Fatalf("collector rows %d", n)
	}
}

func TestImportLegacyIdempotentAndKeepsFiles(t *testing.T) {
	dsn := testDSN(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "collector"), 0o755)
	users := `{"nextId":1002,"accounts":{"device:x":{"id":1001,"key":"device:x","stumbleId":"s1","username":"A","token":"t1","created":"2026-01-01T00:00:00Z","country":"US","region":"eu","equipped":{"a":1},"crowns":2,"xp":3},
"device:y":{"id":1002,"key":"device:y","stumbleId":"s2","username":"B","token":"t2","created":"2026-01-01T00:00:00Z","country":"US","region":"eu","crowns":0,"xp":0}}}`
	os.WriteFile(filepath.Join(dir, "users.json"), []byte(users), 0o600)
	os.WriteFile(filepath.Join(dir, "collector", "20261101T120000.000001-collector.json"), []byte(`{"x":1}`), 0o644)
	ctx := context.Background()
	for i := 0; i < 2; i++ { // second open must not duplicate
		s, err := openStore(ctx, dsn, dir)
		if err != nil {
			t.Fatal(err)
		}
		var na, nc int
		s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM stumble_accounts),(SELECT count(*) FROM stumble_collector)`).Scan(&na, &nc)
		if na != 2 || nc != 1 {
			t.Fatalf("round %d: %d accounts %d collector", i, na, nc)
		}
		if a := s.byToken("t1"); a == nil || a.Crowns != 2 || a.XP != 3 {
			t.Fatalf("account not imported: %+v", a)
		}
		if n := s.get("device:new"); n.ID != 1003+i { // ids are never reused
			t.Fatalf("new id %d, want %d", n.ID, 1003+i)
		}
		s.pool.Exec(ctx, `DELETE FROM stumble_accounts WHERE key='device:new'`)
		s.Close()
	}
	if _, err := os.Stat(filepath.Join(dir, "users.json")); err != nil {
		t.Fatal("users.json removed")
	}
}
