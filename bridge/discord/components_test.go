package bdiscord

import (
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shaped like a SaucyBot reply: a container with text, separators, a one-image gallery and a link button row
const saucyLike = `{"id":"1","channel_id":"2","content":"","flags":32768,"embeds":[],"components":[{"type":17,"accent_color":1942002,"spoiler":false,"components":[
 {"type":10,"content":"### [Some artwork](https://example.com/art/1)"},
 {"type":10,"content":"by **Artist**"},
 {"type":14,"divider":true,"spacing":1},
 {"type":10,"content":"Likes: 12"},
 {"type":12,"items":[{"media":{"url":"https://img.example.com/1.png"},"spoiler":false},{"media":{"url":"https://img.example.com/2.png"},"spoiler":false}]},
 {"type":14,"divider":true,"spacing":1},
 {"type":1,"components":[{"type":2,"style":5,"label":"Open","url":"https://example.com/art/1"},{"type":2,"style":1,"label":"Delete","custom_id":"x"}]}
]}]}`

func TestComponentsToEmbedsSaucyBot(t *testing.T) {
	var m discordgo.Message
	require.NoError(t, json.Unmarshal([]byte(saucyLike), &m))
	require.NotZero(t, m.Flags&discordgo.MessageFlagsIsComponentsV2)

	embeds := componentsToEmbeds(m.Components)
	require.Len(t, embeds, 2)
	e := embeds[0]
	assert.Equal(t, 1942002, e.Color)
	assert.Equal(t, "### [Some artwork](https://example.com/art/1)\nby **Artist**\n───\nLikes: 12\n───\n[Open](https://example.com/art/1)", e.Description)
	require.NotNil(t, e.Image)
	assert.Equal(t, "https://img.example.com/1.png", e.Image.URL)
	assert.Equal(t, "https://img.example.com/2.png", embeds[1].Image.URL, "extra gallery images become image-only embeds")
	assert.Equal(t, discordgo.EmbedTypeRich, e.Type)
}

func TestComponentsToEmbedsSectionAndLoose(t *testing.T) {
	src := `[{"type":10,"content":"loose text"},
	  {"type":9,"components":[{"type":10,"content":"section text"}],"accessory":{"type":11,"media":{"url":"https://img.example.com/t.png"}}}]`
	var m discordgo.Message
	require.NoError(t, json.Unmarshal([]byte(`{"id":"1","flags":32768,"components":`+src+`}`), &m))

	embeds := componentsToEmbeds(m.Components)
	require.Len(t, embeds, 1)
	assert.Equal(t, "loose text\nsection text", embeds[0].Description)
	require.NotNil(t, embeds[0].Thumbnail)
	assert.Equal(t, "https://img.example.com/t.png", embeds[0].Thumbnail.URL)
}

func TestComponentsToEmbedsNothingToShow(t *testing.T) {
	var m discordgo.Message
	require.NoError(t, json.Unmarshal([]byte(`{"id":"1","flags":32768,"components":[{"type":17,"components":[{"type":14}]}]}`), &m))
	assert.Empty(t, componentsToEmbeds(m.Components))
}
