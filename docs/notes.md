# Notes: stumble-guys

## Design

Rounds are deterministic lockstep: every console simulates the round and the Photon server
only relays inputs and keeps the clock. Nothing here simulates the game.

## Status

Work in progress, not deployed, not yet tested against the game. Sign-in, the game config and
matchmaking are stubbed from the contract in [protocol.md](protocol.md). Inventory is
"own everything"; there are no real-money paths.
