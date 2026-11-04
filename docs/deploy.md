# Deploy: stumble-guys

## Image

Built by the repo `Dockerfile`: a `golang:1.26-alpine` build stage (`GOWORK=off`, static
`CGO_ENABLED=0`) and a `gcr.io/distroless/static` runtime running as `nonroot:nonroot`.
It exposes ports 23550 and 23552 and starts `/stumbled -data /data`. Ports are listed in
[ports.md](ports.md).

## Running it

```
GOWORK=off go build -o stumbled ./cmd/stumbled
./stumbled -addr :23550 -health-addr :23552 -data ./data
```

TLS is terminated in front (Traefik with the OpenPak-CA leaf for the three hosts). Requests are
routed by `Host`. Unknown routes are logged with their body and answered `{}`.

| Flag / env | Meaning |
| --- | --- |
| `-database-url` / `STUMBLE_DATABASE_URL` | Postgres URL (required) |
| `-data` | legacy directory with `users.json` and `collector/`, imported once into Postgres, files kept |
| `-shared` | a game config (SharedData JSON) to serve instead of the built-in one |
| `-jwt-key` / `STUMBLE_JWT_KEY` | HMAC key for the Photon and matchmaking tokens |

## Rewards session events

This host sends none. stumbled assigns a Photon room at matchmaking (`OP_<hex>` plus the users list) but the round
runs in Photon Quantum, so joins, leaves and the time spent in the room are invisible here, and players are
partly bots. A 3-minute session cannot be proven from matchmaking alone. Any
`game_sessions.stumble-guys` events must come from the photon host.

## Account erasure

`DELETE /internal/accounts/{account_id}/data` on the health listener (`http://openpak-stumble-guys:23552`, not routed by Traefik), header `Authorization: Bearer <ACCOUNT_ERASE_KEY>`; key unset answers 503. Needs `ACCOUNT_GRPC_ADDR` + `ACCOUNT_INTERNAL_KEY` (read-only core link lookup; the account key is the NSA id token's `sub`, the BAAS user id). Answers `{"service":"stumble-guys","rows":N}`.

Erased: `stumble_accounts` rows of the account, `stumble_collector` bodies containing its Stumble id, and in memory the matchmaker tickets and queued requests carrying its numeric user id. Kept: other collector bodies (unattributed client analytics); `device:<id>` accounts (no OpenPak link exists to find them by).
