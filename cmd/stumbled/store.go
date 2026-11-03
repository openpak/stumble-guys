package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// store keeps accounts and the kept analytics bodies in Postgres. Nothing is held in memory.
type store struct {
	pool *pgxpool.Pool
}

const schema = `
CREATE SEQUENCE IF NOT EXISTS stumble_account_id START 1001;
CREATE TABLE IF NOT EXISTS stumble_accounts (
  id         integer PRIMARY KEY,
  key        text NOT NULL UNIQUE,
  stumble_id text NOT NULL,
  username   text NOT NULL,
  token      text NOT NULL UNIQUE,
  created    text NOT NULL,
  country    text NOT NULL,
  region     text NOT NULL,
  equipped   jsonb,
  crowns     integer NOT NULL DEFAULT 0,
  xp         integer NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS stumble_collector (
  name        text PRIMARY KEY,
  received_at timestamptz NOT NULL DEFAULT now(),
  host        text NOT NULL,
  body        bytea NOT NULL
);`

// openStore connects to Postgres (small pool), creates the schema and imports the legacy
// files found in importDir (users.json, collector/) once. The files are left in place.
func openStore(ctx context.Context, url, importDir string) (*store, error) {
	if url == "" {
		return nil, errors.New("STUMBLE_DATABASE_URL is not set: accounts live in Postgres")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 3
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	s := &store{pool}
	if importDir != "" {
		if err := s.importLegacy(ctx, importDir); err != nil {
			pool.Close()
			return nil, fmt.Errorf("import %s: %w", importDir, err)
		}
	}
	return s, nil
}

func (s *store) Close() { s.pool.Close() }

// importLegacy copies users.json and collector/*.json into the tables. Idempotent
// (ON CONFLICT DO NOTHING); the id sequence is moved past the file's nextId.
func (s *store) importLegacy(ctx context.Context, dir string) error {
	var legacy struct {
		NextID int                 `json:"nextId"`
		ByKey  map[string]*account `json:"accounts"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "users.json"))
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &legacy); err != nil {
			return err
		}
	case !os.IsNotExist(err):
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	accs := 0
	for _, a := range legacy.ByKey {
		var eq any
		if len(a.Equipped) > 0 {
			eq = string(a.Equipped)
		}
		ct, err := tx.Exec(ctx, `INSERT INTO stumble_accounts
(id,key,stumble_id,username,token,created,country,region,equipped,crowns,xp)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11) ON CONFLICT DO NOTHING`,
			a.ID, a.Key, a.StumbleID, a.Username, a.Token, a.Created, a.Country, a.Region, eq, a.Crowns, a.XP)
		if err != nil {
			return err
		}
		accs += int(ct.RowsAffected())
	}
	if legacy.NextID > 0 {
		// Next nextval() must be above every id handed out before.
		if _, err := tx.Exec(ctx, `SELECT setval('stumble_account_id', GREATEST($1::bigint, (SELECT last_value FROM stumble_account_id), 1001))`, legacy.NextID); err != nil {
			return err
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "collector", "*.json"))
	cols := 0
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		name := filepath.Base(f)
		// <timestamp>-<host label>.json
		host := strings.TrimSuffix(name, ".json")
		if i := strings.IndexByte(host, '-'); i >= 0 {
			host = host[i+1:]
		}
		at := time.Now().UTC()
		if i := strings.IndexByte(name, '-'); i > 0 {
			if t, err := time.Parse("20060102T150405.000000", name[:i]); err == nil {
				at = t
			}
		}
		ct, err := tx.Exec(ctx, `INSERT INTO stumble_collector (name,received_at,host,body) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, name, at, host, body)
		if err != nil {
			return err
		}
		cols += int(ct.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	var na, nc int
	s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM stumble_accounts),(SELECT count(*) FROM stumble_collector)`).Scan(&na, &nc)
	log.Printf("[stumble] postgres store ready: import from %s: %d/%d accounts, %d/%d collector files added; tables hold %d accounts, %d collector bodies",
		dir, accs, len(legacy.ByKey), cols, len(files), na, nc)
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

const accCols = `id,key,stumble_id,username,token,created,country,region,equipped,crowns,xp`

func scanAccount(row pgx.Row) (*account, error) {
	a := &account{}
	var eq []byte
	if err := row.Scan(&a.ID, &a.Key, &a.StumbleID, &a.Username, &a.Token, &a.Created, &a.Country, &a.Region, &eq, &a.Crowns, &a.XP); err != nil {
		return nil, err
	}
	if len(eq) > 0 {
		a.Equipped = json.RawMessage(eq)
	}
	return a, nil
}

// get returns the account for key, creating it on first sight.
func (s *store) get(key string) *account {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if a, err := scanAccount(s.pool.QueryRow(ctx, `SELECT `+accCols+` FROM stumble_accounts WHERE key=$1`, key)); err == nil {
		return a
	}
	var id int
	if err := s.pool.QueryRow(ctx, `SELECT nextval('stumble_account_id')`).Scan(&id); err != nil {
		log.Printf("[stumble] account id: %v", err)
		return nil
	}
	// ON CONFLICT (key): a concurrent first login of the same key wins; re-read below.
	s.pool.Exec(ctx, `INSERT INTO stumble_accounts (id,key,stumble_id,username,token,created,country,region)
VALUES ($1,$2,$3,$4,$5,$6,'US','eu') ON CONFLICT (key) DO NOTHING`,
		id, key, randHex(12), "Stumbler"+itoa(id), randHex(24), nowISO())
	a, err := scanAccount(s.pool.QueryRow(ctx, `SELECT `+accCols+` FROM stumble_accounts WHERE key=$1`, key))
	if err != nil {
		log.Printf("[stumble] account %q: %v", key, err)
		return nil
	}
	return a
}

func (s *store) byToken(tok string) *account {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, err := scanAccount(s.pool.QueryRow(ctx, `SELECT `+accCols+` FROM stumble_accounts WHERE token=$1`, tok))
	if err != nil {
		return nil
	}
	return a
}

func (s *store) update(a *account, f func(*account)) {
	f(a)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var eq any
	if len(a.Equipped) > 0 && json.Valid(a.Equipped) {
		eq = string(a.Equipped)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE stumble_accounts SET username=$2,country=$3,region=$4,equipped=$5::jsonb,crowns=$6,xp=$7 WHERE id=$1`,
		a.ID, a.Username, a.Country, a.Region, eq, a.Crowns, a.XP); err != nil {
		log.Printf("[stumble] update account %d: %v", a.ID, err)
	}
}

// keepCollector stores one analytics body whole.
func (s *store) keepCollector(host string, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	label := host
	if p := strings.SplitN(host, ".", 3); len(p) > 1 {
		label = p[1]
	}
	name := time.Now().UTC().Format("20060102T150405.000000") + "-" + label + ".json"
	if _, err := s.pool.Exec(ctx, `INSERT INTO stumble_collector (name,host,body) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, name, label, body); err != nil {
		log.Printf("[stumble] keep collector: %v", err)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
