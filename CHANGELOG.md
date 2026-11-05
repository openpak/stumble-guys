# Changelog — stumble-guys

## v0.3.5 — 2026-11-05

- RoundLevels_v2 lists the two shipped race levels in each pool. With the list empty the client's round setup raised "The sequence requires at least one element" (read from the guest's heap) and dropped the player (prds#12).

## v0.4.0 — 2026-11-05

- Link at first use (prds#131): `/user/login` answers 403 `account_not_linked` ("Link your OpenPak account to play online") for a Switch credential no active OpenPak account owns, and for a login with no NSA token. With no account core configured (development) the old behaviour stands.
- Name and country come from the linked core account, cached five minutes and dropped at once on the core's `profile_updated`, `link_unlinked`, `account_deleted` and `account_banned` events. `stumble_accounts.username` and `.country` are dropped at startup (idempotent); crowns, xp and equipped cosmetics stay as game progress, covered by the erasure endpoint.

## v0.3.4 — 2026-11-05

- MatchmakingPools in the shared config lists three pools. The client picks a pool by id and falls back to the second entry when none matches; with an empty list the round setup threw and the player was dropped to the title screen (prds#12).

## v0.3.3 — 2026-11-05

- Photon app id mapping now names the Switch platform and a version range. The client sent an empty app id to Photon with the
  unbounded mapping, so the Quantum room was never recognised and the round never started (prds#12).

## v0.3.2 — 2026-11-05

- The API now matches a trailing slash: Stumble Guys calls `/matchmaking/create/signature/latest/`, which fell through to the
  empty default answer, so the client never held a ticket and posted `{}` to the matchmaker (prds#12).

## v0.3.1 — 2026-11-05

- Release-order repair (prds#120). v0.3.0 (account erasure, below) was tagged on its own commit, then v0.2.2 and v0.2.3
  were cut from main afterwards, so ghcr `:latest` stayed at 0.2.3 and the erase endpoint never reached production.
  main holds everything (erasure plus the matchmaker fixes); this tag is the first build that carries both and is
  newer than every earlier tag. No code change.

## v0.2.3 — 2026-11-04

- The newest-ticket fallback is kept under its own key: the API call that signs the ticket carried no session the
  matchmaker could match, so v0.2.2's lookups found nothing.

## v0.2.2 — 2026-11-04

- Matchmaker `POST /request` with an empty body: the signed ticket may ride in `Authorization` (claim `p`); the fallback
  chain is that, then the session's last ticket, then the newest ticket signed (the game's matchmaker session did not
  match the API session in v0.2.1's first test).

## v0.3.0 — 2026-11-04

- Account erasure (prds#120): `DELETE /internal/accounts/{account_id}/data` on the health listener (23552; `Authorization: Bearer $ACCOUNT_ERASE_KEY`, unset = 503). Looks up the account's BAAS ids in the core (`ACCOUNT_GRPC_ADDR`, `ACCOUNT_INTERNAL_KEY`, read-only ListAccountLinks; unconfigured = 500) and deletes the `stumble_accounts` rows keyed by them (profile, token, cosmetics, crowns, xp), the kept `stumble_collector` analytics bodies that contain the account's Stumble id, and the matchmaker's in-memory tickets and queued requests for it. Accounts created without an NSA token (`device:<id>` keys) cannot be tied to an OpenPak account and are not touched.

## v0.2.1 — 2026-11-04

- `POST /request` on the matchmaker: Stumble Guys on the Switch posts `{}` instead of the signed ticket. The request
  now falls back to the ticket last signed for that session (kept in memory, short-lived) and logs the request header
  names, so the real wire shape can be measured.

## v0.2.0 — 2026-11-03

**Accounts and kept analytics live in Postgres.** `users.json` and the `collector/` files are replaced by the tables
`stumble_accounts` and `stumble_collector` (small pool, 3 connections), so one database backup covers the service.

- New `STUMBLE_DATABASE_URL` (or `-database-url`). Without it the service refuses to start; nothing is kept in memory or on disk.
- `-data` now only names a legacy directory: at start `users.json` and `collector/*.json` found there are imported
  once (idempotent, `ON CONFLICT DO NOTHING`, the id sequence continues after the file's `nextId`). The files are never
  deleted or modified. The import logs added/total counts.
- Restart test and import test (`STUMBLE_TEST_DATABASE_URL`, skipped when unset).
