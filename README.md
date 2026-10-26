# Stumble Guys

OpenPak's Stumble Guys Server.

| | |
| --- | --- |
| Game | [Stumble Guys](https://openpak.org/titles#t-72378) |
| Network | Nintendo Switch |
| Title ID | `0100A9E01AE9A000` |
| Licence | AGPL-3.0-only |

## What it serves

| Host | What | Served by |
| --- | --- | --- |
| `api.stumbleguys.com` | Sign-in, profile, inventory, game config, rewards, friends, party | `stumbled` |
| `matchmaking.stumbleguys.com` | Room assignment | `stumbled` |
| `localization.stumbleguys.com` | Localization | `stumbled` |
| Photon Realtime | Rounds (Quantum 2 lockstep) | [`photon`](https://github.com/openpak/photon) |

## Run

```sh
GOWORK=off go build -o stumbled ./cmd/stumbled
./stumbled -addr :23550 -health-addr :23552 -data ./data
```

| Flag / env | Meaning |
| --- | --- |
| `-data` | directory for `users.json` |
| `-shared` | a game config (SharedData JSON) to serve instead of the built-in one |
| `-jwt-key` / `STUMBLE_JWT_KEY` | HMAC key for the Photon and matchmaking tokens |

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)
- [Ports](docs/ports.md)
- [Provenance](docs/provenance.md)

Live compatibility status: [openpak.org/titles#t-72378](https://openpak.org/titles#t-72378).

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
