# Ports

| Port | Use |
| --- | --- |
| 23550 | plain HTTP: `api.`, `matchmaking.` and `localization.stumbleguys.com`, routed by `Host` (behind the TLS terminator) |
| 23551 | Pusher-protocol websocket (planned) |
| 23552 | `/health` |
| 23553–23559 | reserved |

Rounds use the photon server's ports; this service publishes none.
