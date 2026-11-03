package main

import (
	"context"
	"os"
	"testing"
)

// testDSN returns STUMBLE_TEST_DATABASE_URL, or skips. The tables are emptied first.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("STUMBLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("STUMBLE_TEST_DATABASE_URL not set")
	}
	st, err := openStore(context.Background(), dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	st.pool.Exec(context.Background(), `TRUNCATE stumble_accounts, stumble_collector; ALTER SEQUENCE stumble_account_id RESTART WITH 1001`)
	st.Close()
	return dsn
}
