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
| `-data` | directory for `users.json` |
| `-shared` | a game config (SharedData JSON) to serve instead of the built-in one |
| `-jwt-key` / `STUMBLE_JWT_KEY` | HMAC key for the Photon and matchmaking tokens |
