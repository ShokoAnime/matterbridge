package bdiscord

import "testing"

func TestReplySnippet(t *testing.T) {
	cases := []struct {
		in           string
		files, embed bool
		want         string
	}{
		{"hello there\nsecond line", false, false, "hello there"},
		{"see [this](x) *now*", false, false, "see \\[this\\](x) \\*now\\*"},
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
