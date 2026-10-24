package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

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
func (s *server) userResponse(a *account, version, deviceID string) map[string]any {
	photon := s.photonJWT(a)
	user := map[string]any{
		"Id":                     a.ID,
		"StumbleId":              a.StumbleID,
		"DeviceId":               deviceID,
		"ScopelyJWT":             "",
		"Username":               a.Username,
		"Country":                a.Country,
		"Region":                 a.Region,
		"Token":                  a.Token,
		"Version":                version,
		"VersionResult":          "",
		"Friends":                []any{},
		"Skins":                  []any{},
		"SkinVariants":           []any{},
		"Rewards":                []any{},
		"Balances":               []any{},
		"Created":                a.Created,
		"LastLogin":              nowISO(),
		"SkillRating":            0,
		"Crowns":                 a.Crowns,
		"HiddenRating":           0,
		"Experience":             a.XP,
		"NewsVersion":            0,
		"AvailableNewsVersion":   0,
		"LatestNewsIdBackend":    0,
		"Emotes":                 []any{},
		"Animations":             []any{},
		"Footsteps":              []any{},
		"ActionEmotes":           []any{},
		"HasBattlePass":          false,
		"PassTokens":             0,
		"FreePassRewards":        []any{},
		"PremiumPassRewards":     []any{},
		"KidFriendlyMode":        0,
		"Age":                    18,
		"TermsOfServiceAccepted": true,
		"FeaturesEnabled":        []any{},
		"SecondsSinceCreated":    secondsSince(a.Created),
		"PhotonJwt":              photon,
		"NativePlatformId":       "",
		"NativePlatformName":     "Switch",
		"PlatformType":           0,
		"Inventory":              []any{},
		"InventoryShards":        map[string]any{},
		"HelpshiftUserIdentity":  "",
		"OfferSequenceState":     []any{},
	}
	resp := map[string]any{
		"User":                   user,
		"TermsOfServiceAccepted": true,
		"PhotonJwt":              photon,
		"ScopelyId":              "",
		"FeatureFlags":           []any{},
		"HelpshiftUserIdentity":  "",
	}
	if len(a.Equipped) > 0 {
		resp["EquippedCosmetics"] = a.Equipped
	}
	return resp
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
