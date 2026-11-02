package bdiscord

import (
	"encoding/json"
	"testing"

	"github.com/matterbridge-org/matterbridge/bridge/config"
	"github.com/stretchr/testify/assert"
)

func TestReplySnippet(t *testing.T) {
	cases := []struct {
		in           string
		files, embed bool
		want         string
	}{
		{"hello there\nsecond line", false, false, "hello there"},
		{"see [this](x) *now*", false, false, "see \\[this\\](x) \\*now\\*"},
		{"One for <@308830889281060864> https://www.ebay.com/itm/ABSURD-Studio-Tayama-Figure", false, false, "One for @… 🔗 ebay.com"},
		{"look <:shoko:123> at <#456>", false, false, "look :shoko: at @…"},
		{"a > b", false, false, "a > b"},
		{"", true, false, "attachment"},
		{"", false, true, "embed"},
		{"", false, false, "message"},
	}
	for _, c := range cases {
		if got := replySnippet(c.in, c.files, c.embed); got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "é"
	}
	if got := []rune(replySnippet(long, false, false)); len(got) != 81 {
		t.Errorf("long snippet is %d runes", len(got))
	}
}

func TestReplyPing(t *testing.T) {
	assert.True(t, replyPing(&config.Message{}), "a reply pings by default")
	assert.False(t, replyPing(&config.Message{Extra: map[string][]any{"reply_ping": {false}}}), "a silent reply stays silent")
	assert.True(t, replyPing(&config.Message{Extra: map[string][]any{"reply_ping": {true}}}))

	var m config.Message // as an API client sends it
	assert.NoError(t, json.Unmarshal([]byte(`{"text":"x","Extra":{"reply_ping":[false]}}`), &m))
	assert.False(t, replyPing(&m))
}
