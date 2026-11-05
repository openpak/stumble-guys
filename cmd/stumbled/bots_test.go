package main

import "testing"

func TestBotDefinitionsUseTheServedCosmetics(t *testing.T) {
	s := &server{shared: []byte(`{"Skins_v4":[{"SkinID":"SKIN1"},{"SkinID":"SKIN2"}],"Footsteps_v2":[{"ID":"footsteps_smoke"}],"Animations_v2":[{"ID":"animation1"}],"Emotes_v2":[{"ID":"emote_a"},{"ID":"emote_b"}]}`)}
	bots := s.botDefinitions()
	if len(bots) != 24 || bots[1]["SkinId"] != "SKIN2" || bots[0]["FootstepsId"] != "footsteps_smoke" {
		t.Fatalf("bots = %v", bots[:2])
	}
	if got := (&server{shared: []byte(`{}`)}).botDefinitions(); len(got) != 0 {
		t.Fatalf("no cosmetics should give no bots, got %d", len(got))
	}
}
