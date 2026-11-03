# Changelog — stumble-guys

## v0.2.0 — 2026-11-03

**Accounts and kept analytics live in Postgres.** `users.json` and the `collector/` files are replaced by the tables
`stumble_accounts` and `stumble_collector` (small pool, 3 connections), so one database backup covers the service.

- New `STUMBLE_DATABASE_URL` (or `-database-url`). Without it the service refuses to start; nothing is kept in memory or on disk.
- `-data` now only names a legacy directory: at start `users.json` and `collector/*.json` found there are imported
  once (idempotent, `ON CONFLICT DO NOTHING`, the id sequence continues after the file's `nextId`). The files are never
  deleted or modified. The import logs added/total counts.
- Restart test and import test (`STUMBLE_TEST_DATABASE_URL`, skipped when unset).
