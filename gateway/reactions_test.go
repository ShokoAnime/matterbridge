package gateway

import (
	"testing"

	"github.com/matterbridge-org/matterbridge/bridge"
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

func TestReactionTargetResolvesCopies(t *testing.T) {
	gw := newTestGateway("g", "discord.a", "api.b")
	disc, api := gw.Bridges["discord.a"], gw.Bridges["api.b"]
	disc.Protocol, disc.Name, api.Protocol, api.Name = "discord", "a", "api", "b"
	dch := &config.ChannelInfo{ID: "ID:1discord.a"}
	ach := &config.ChannelInfo{ID: "apiapi.b"}
	// a Fluxer (api) message 500 was bridged to Discord as 900
	gw.Messages.Add("api 500", []*BrMsgID{{disc, "discord 900", dch.ID}})
	// a Discord message 700 was bridged to the API (which returned its source id)
	gw.Messages.Add("discord 700", []*BrMsgID{{api, "api discord 700", ach.ID}})

	cases := []struct {
		from, id string
		dest     *bridge.Bridge
		ch       *config.ChannelInfo
		want     string
	}{
		{"discord", "900", api, ach, "500"},         // reacting on Discord to the copy of a Fluxer message
		{"discord", "700", api, ach, "discord 700"}, // reacting on Discord to a Discord message
		{"api", "500", disc, dch, "900"},            // reacting on Fluxer to a Fluxer message
		{"api", "discord 700", disc, dch, "700"},    // reacting on Fluxer to the copy of a Discord message
		{"discord", "123", api, ach, ""},            // unknown
	}
	for _, c := range cases {
		got := gw.reactionTargetID(&config.Message{Protocol: c.from, ID: c.id}, c.dest, c.ch)
		if got != c.want {
			t.Errorf("%s %s -> %s: got %q want %q", c.from, c.id, c.dest.Protocol, got, c.want)
		}
	}
}
