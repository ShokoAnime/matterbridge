package gateway

import "testing"

func TestEmojiSprintKeepsMarkup(t *testing.T) {
	if got := emojiSprint("<:smile:123> :smile:"); got != "<:smile:123> 😄" {
		t.Errorf("got %q", got)
	}
	if got := stripCustomEmoji("a <a:party:5> b <:x_y:6>"); got != "a :party: b :x_y:" {
		t.Errorf("got %q", got)
	}
}
