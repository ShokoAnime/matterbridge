package bdiscord

import "testing"

func TestReplaceShortcodes(t *testing.T) {
	lookup := func(n string) string {
		if n == "shoko" || n == "wave" {
			return "<:" + n + ":1>"
		}
		return ""
	}
	cases := map[string]string{
		"hi :shoko:":           "hi <:shoko:1>",
		":shoko::wave:":        "<:shoko:1><:wave:1>",
		"<:shoko:99> stays":    "<:shoko:99> stays",
		"<a:shoko:99> stays":   "<a:shoko:99> stays",
		"unknown :nope: kept":  "unknown :nope: kept",
		"time 10:30:00 :wave:": "time 10:30:00 <:wave:1>",
		"x:nope:shoko: y":      "x:nope<:shoko:1> y",
	}
	for in, want := range cases {
		if got := replaceShortcodes(in, lookup); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestWebhookUsername(t *testing.T) {
	cases := map[string]string{
		"Discord Mod": "Discоrd Mod",
		"clyde":       "clydе",
		"everyone":    "everyone​",
		"  ":          "unknown",
		"ᴿᵋᵛᶺꭩ":       "ᴿᵋᵛᶺꭩ",
	}
	for in, want := range cases {
		if got := webhookUsername(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "ᴿ"
	}
	if got := []rune(webhookUsername(long)); len(got) != 80 {
		t.Errorf("long name cut to %d runes", len(got))
	}
}
