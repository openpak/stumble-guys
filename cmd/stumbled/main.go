// Command stumbled answers Stumble Guys' backend (api.stumbleguys.com), matchmaker
// (matchmaking.stumbleguys.com) and localization host for the Switch build (0100A9E01AE9A000)
// when a console is redirected to OpenPak.
//
// Gameplay is Photon Quantum on the photon server; this service is the developer's REST side:
// sign-in, the game config, profile and inventory, matchmaking tickets and room assignment.
// It listens on plain HTTP behind the TLS terminator. Routes it does not know yet are logged with
// their body and answered with an empty JSON object, so a capture shows what the client asks next.
//
// The contract was read out of the Switch executable; see docs/protocol.md.
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
)

var (
	addr       = flag.String("addr", ":23550", "plain HTTP listen address (behind the TLS terminator)")
	healthAddr = flag.String("health-addr", ":23552", "health listen address")
	dataDir    = flag.String("data", "./data", "directory for the user store")
	sharedFile = flag.String("shared", "", "game config (SharedData JSON) to serve instead of the built-in one")
	jwtKey     = flag.String("jwt-key", "", "HMAC key for the Photon and matchmaking tokens (env STUMBLE_JWT_KEY)")
)

func main() {
	flag.Parse()
	s, err := newServer(*dataDir, *sharedFile, keyFromEnv(*jwtKey))
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		h := http.NewServeMux()
		h.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
		log.Printf("[stumble] health on %s", *healthAddr)
		log.Fatal(http.ListenAndServe(*healthAddr, h))
	}()
	log.Printf("[stumble] serving HTTP on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, s))
}
