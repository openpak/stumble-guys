# Protocol

What the Switch client (update v393216, client 0.81.0.21) expects, read from the client.

## Hosts

`https://api.stumbleguys.com`, `https://matchmaking.stumbleguys.com` and
`https://localization.stumbleguys.com`, from the client's live environment settings. JSON bodies;
the client may gzip a request body.

## Authentication

- Every backend request carries the console's NSA id token in `x-authorization`.
- After login, an `authorization` header carries a JSON object that includes the session `Token`
  the login returned (plus device and timestamp fields).

## Sign-in

`POST /user/login` with `{Id, DeviceId, Version, FacebookId, AppleId, GoogleId, Timestamp,
AdvertisingId, StumbleId, ScopelyId, NativeId, Hash}`. The answer is
`{User, TermsOfServiceAccepted, PhotonJwt, ScopelyId, FeatureFlags, EquippedCosmetics,
HelpshiftUserIdentity}`; `User` holds the profile (Id, Username, Country, Region, Token,
balances, cosmetics, inventory, battle pass...).

`GET /user/config` returns the game config (levels, round levels, matchmaking pools, cosmetics,
shop, Photon settings including the AppId mapping and regions).

## Matchmaking

1. `api`: `/matchmaking/create/signature/latest` with `{Region, GameMode, Variant,
   IncludedLocally, InclusionList, ExclusionList, StumblePartyParameters}` returns
   `{Parameters, Signature}`; `Parameters` is the matchmaking request (RequestId, UserId,
   Region, GameMode, Variant, MaxPlayers, ...).
2. `matchmaking`: `POST /request` with that object. `GET /request/{RequestId}` is polled and
   `POST /request/{RequestId}/cancel` cancels. A result is `{Status, Room, Users, Ticket,
   ClientAppVersion, Region}`; Status is 0 Error, 1 InProgress, 2 Matched, 3 NotFound,
   4 Expired. On Matched the client joins the named Photon room.

## Rounds (Photon Quantum)

Quantum protocol messages travel as Photon RaiseEvent payloads: event 100 (reliable) and 101
(unreliable) from the client; the client accepts events 100–102. A payload is a little-endian,
LSB-first bit stream of `[message id: 8 bits][message fields]...`. Message ids:

| Id | Message | Fields |
| --- | --- | --- |
| 1 | Join | string Id, string ProtocolVersion, int PlayerSlots, int InitialTick, int PlayerCount, (newer protocol) bool Rejoin |
| 2 | Joined | bool Confirmed, int Status, int[] PlayerSlots |
| 3 | SessionConfig | bool Requested, DeterministicSessionConfig |
| 4 | RuntimeConfig | bool Requested, byte[] Config |
| 5 | SimulationStart | bool, double, byte[], DeterministicSessionConfig |
| 6 | SimulationStop | int |
| 7 | ClockCorrect | double ServerSeconds, double ServerTimeScale |
| 8 | TickChecksum | int Tick, ulong Checksum |
| 9 | TickChecksumError | (error record) |
| 10 | RttUpdate | int |
| 11 | SetPlayerData | int Index, byte[] Data |
| 12 | Disconnect | string Reason |
| 13 | FrameSnapshot | int Tick, int TotalSize, bool Last, byte[] Data |
| 14 | Command | int Index, byte[] Data |
| 15 | TickChecksumErrorFrameDump | int Frame, int Block, int BlockCount, int ActorId, byte[] Data |
| 16 | InputMissing | int PlayerCount, TickMissing[] {int Tick, ulong Mask} |
| 17 | FrameSnapshotRequest | int ReferenceTick |
| 21 | StartRequest | — |

Primitives: bool = 1 bit; int/uint = 32 bits; long/ulong = 64 bits (low word first); double = 8
raw bytes; string = 1 bit "is null", then u16 byte length and UTF-8 bytes; byte[] and int[] = 1
bit "present", u16 count, elements.

## From the previous README

Stumble Guys on Nintendo Switch (`0100A9E01AE9A000`) plays online through these services:

| What | Where | Served by |
| --- | --- | --- |
| Sign-in, profile, inventory, game config, rewards, friends, party | `api.stumbleguys.com` (REST, JSON) | this repo (`stumbled`) |
| Matchmaking (room assignment) | `matchmaking.stumbleguys.com` | this repo (`stumbled`) |
| Localization | `localization.stumbleguys.com` | this repo (`stumbled`) |
| Rounds | Photon Quantum 2 on Photon Realtime | [`photon`](https://github.com/openpak/photon) |
