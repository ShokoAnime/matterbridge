package bdiscord

import "testing"

func TestRestoreChannelMentions(t *testing.T) {
	if got := restoreChannelMentions("go to \x00ch123\x00 now"); got != "go to <#123> now" {
		t.Errorf("got %q", got)
	}
}
