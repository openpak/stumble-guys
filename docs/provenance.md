# Provenance

Where each fact in [protocol.md](protocol.md) came from, per the
[clean-room policy](https://docs.openpak.org/contributors/clean-room/). Only the owner's own game
client was examined. No server code of the developer, Scopely or Exit Games was seen or used.

| Fact | Source |
| --- | --- |
| The three hosts and the client version | the live environment settings asset in the Switch RomFS |
| `x-authorization` carries the NSA id token; `authorization` JSON carries the session Token | the request-signing code and its literals, Switch executable |
| Login body field names and the response shape | the login request builder's literals and the response type's fields (IL2CPP metadata) |
| Route paths | string literals in the Switch executable |
| Matchmaking request/result types and status values | type and enum definitions in the IL2CPP metadata |
| Quantum event codes 100/101 (sent) and 100–102 (accepted) | the client's protocol send and receive code, Switch executable |
| Quantum message ids and field order | the client's message registry and each message's Serialize method, Switch executable |
| Bit stream layout | the client's bit stream reader and writer, Switch executable |
