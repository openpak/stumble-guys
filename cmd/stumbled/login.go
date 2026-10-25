package main

import (
	"crypto/hmac"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

//go:embed user.json
var userTemplate []byte

// loginRequest is Backend.Login's body. Hash is a digest under a client-side constant; it proves
// nothing about the player, so it is not checked.
type loginRequest struct {
	Id, DeviceId, Version, StumbleId, NativeId string
	Timestamp                                  any
}

func (s *server) login(w http.ResponseWriter, r *http.Request, body []byte) {
	var req loginRequest
	json.Unmarshal(body, &req)
	id := requestIdentity(r)
	key := id.NSAID
	if key == "" {
		key = "device:" + req.DeviceId
	}
	if key == "device:" {
		log.Printf("[stumble] login without an NSA token or device id")
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	a := s.users.get(key)
	log.Printf("[stumble] login user=%d version=%s nsa=%t", a.ID, req.Version, id.NSAID != "")
	writeJSON(w, http.StatusOK, s.userResponse(a, req.Version, req.DeviceId))
}

// userResponse is the client's BackendUserResponse: {User, TermsOfServiceAccepted, PhotonJwt,
// ScopelyId, FeatureFlags, EquippedCosmetics, HelpshiftUserIdentity}.
// userResponse is a struct, not a map: the client only takes the user from a response whose text
// starts with {"User": (Backend.HandleResponse checks the prefix), and Go sorts map keys.
type userResponse struct {
	User                   map[string]any `json:"User"`
	TermsOfServiceAccepted bool           `json:"TermsOfServiceAccepted"`
	PhotonJwt              string         `json:"PhotonJwt"`
	ScopelyId              string         `json:"ScopelyId"`
	FeatureFlags           []any          `json:"FeatureFlags"`
	HelpshiftUserIdentity  string         `json:"HelpshiftUserIdentity"`
	EquippedCosmetics      any            `json:"EquippedCosmetics"`
}

func (s *server) userResponse(a *account, version, deviceID string) userResponse {
	photon := s.photonJWT(a)
	// Every field of the client's User, from the type's own shape (user.json): a null nested object
	// (BattlePass, XpRoad, Settings, ...) throws inside the client's login handling.
	var user map[string]any
	json.Unmarshal(userTemplate, &user)
	for k, v := range map[string]any{
		"Id":                     a.ID,
		"StumbleId":              a.StumbleID,
		"DeviceId":               deviceID,
		"Username":               a.Username,
		"Country":                a.Country,
		"Region":                 a.Region,
		"Token":                  a.Token,
		"Version":                version,
		"Created":                a.Created,
		"LastLogin":              nowISO(),
		"Crowns":                 a.Crowns,
		"Experience":             a.XP,
		"Age":                    18,
		"TermsOfServiceAccepted": true,
		"SecondsSinceCreated":    secondsSince(a.Created),
		"PhotonJwt":              photon,
		"NativePlatformName":     "Switch",
	} {
		user[k] = v
	}
	equipped := user["EquippedCosmetics"]
	if len(a.Equipped) > 0 {
		equipped = a.Equipped
		user["EquippedCosmetics"] = a.Equipped
	}
	return userResponse{
		User:                   user,
		TermsOfServiceAccepted: true,
		PhotonJwt:              photon,
		FeatureFlags:           []any{},
		EquippedCosmetics:      equipped,
	}
}

func secondsSince(iso string) int {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return 0
	}
	return int(time.Since(t).Seconds())
}

// photonJWT is the token the client hands Photon's custom authentication (auth type "jwt").
func (s *server) photonJWT(a *account) string {
	return s.signJWT(map[string]any{
		"sub":  a.StumbleID,
		"uid":  a.ID,
		"name": a.Username,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
}

func (s *server) signJWT(claims map[string]any) string {
	enc := base64.RawURLEncoding
	head := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	msg := head + "." + enc.EncodeToString(body)
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(msg))
	return msg + "." + enc.EncodeToString(m.Sum(nil))
}

// equip stores the client's UserEquippedCosmeticsDto and echoes it back.
func (s *server) equip(w http.ResponseWriter, r *http.Request, body []byte) {
	a := s.users.byToken(requestIdentity(r).Session)
	if a == nil || !json.Valid(body) {
		logRequest("api equip (no session)", r, body)
		writeJSON(w, http.StatusOK, json.RawMessage(body))
		return
	}
	s.users.update(a, func(a *account) { a.Equipped = append(json.RawMessage(nil), body...) })
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
