# stumble-guys — OpenPak's backend for Stumble Guys

Stumble Guys on Nintendo Switch (`0100A9E01AE9A000`) plays online through these services:

| What | Where | Served by |
| --- | --- | --- |
| Sign-in, profile, inventory, game config, rewards, friends, party | `api.stumbleguys.com` (REST, JSON) | this repo (`stumbled`) |
| Matchmaking (room assignment) | `matchmaking.stumbleguys.com` | this repo (`stumbled`) |
| Localization | `localization.stumbleguys.com` | this repo (`stumbled`) |
| Rounds | Photon Quantum 2 on Photon Realtime | [`photon`](https://github.com/openpak/photon) |

Rounds are deterministic lockstep: every console simulates the round and the Photon server
only relays inputs and keeps the clock. Nothing here simulates the game.

## Status

Work in progress, not deployed, not yet tested against the game. Sign-in, the game config and
matchmaking are stubbed from the contract in [docs/protocol.md](docs/protocol.md). Inventory is
"own everything"; there are no real-money paths.

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

## License

AGPL-3.0-only. See [LICENSE](LICENSE).
