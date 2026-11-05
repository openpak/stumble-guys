package main

import (
	"encoding/json"
	"fmt"
)

// botDefinitions answers GET /bots: the players the client fills a lobby with. Each takes its cosmetics from the
// game config being served (first entries of its own lists), so every id exists in the client's catalog.
func (s *server) botDefinitions() []map[string]any {
	var c struct {
		Skins      []struct{ SkinID string }
		Footsteps  []struct{ ID string }
		Animations []struct{ ID string }
		Emotes     []struct{ ID string }
	}
	var raw struct {
		Skins      json.RawMessage `json:"Skins_v4"`
		Footsteps  json.RawMessage `json:"Footsteps_v2"`
		Animations json.RawMessage `json:"Animations_v2"`
		Emotes     json.RawMessage `json:"Emotes_v2"`
	}
	json.Unmarshal(s.shared, &raw)
	json.Unmarshal(raw.Skins, &c.Skins)
	json.Unmarshal(raw.Footsteps, &c.Footsteps)
	json.Unmarshal(raw.Animations, &c.Animations)
	json.Unmarshal(raw.Emotes, &c.Emotes)
	if len(c.Skins) == 0 || len(c.Footsteps) == 0 || len(c.Animations) == 0 || len(c.Emotes) == 0 {
		return []map[string]any{} // a config without cosmetics: the client adds its own built-in bots
	}
	bots := make([]map[string]any, 0, 24)
	for i := 0; i < 24; i++ {
		emotes := []string{}
		for j := 0; j < 4 && j < len(c.Emotes); j++ {
			emotes = append(emotes, c.Emotes[(i+j)%len(c.Emotes)].ID)
		}
		bots = append(bots, map[string]any{
			"Id":             i + 1,
			"Username":       fmt.Sprintf("Stumbler%d", 2000+i),
			"SkinId":         c.Skins[i%len(c.Skins)].SkinID,
			"FootstepsId":    c.Footsteps[i%len(c.Footsteps)].ID,
			"AnimationId":    c.Animations[i%len(c.Animations)].ID,
			"SocialEmoteIds": emotes,
		})
	}
	return bots
}
