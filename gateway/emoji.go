package gateway

import (
	"regexp"

	"github.com/kyokomi/emoji/v2"
)

// customEmojiRE matches Discord/Fluxer custom emoji markup: <:name:id> and animated <a:name:id>.
var customEmojiRE = regexp.MustCompile(`<a?:(\w+):\d+>`)

// keepsCustomEmoji reports whether a destination understands <:name:id> markup. "api" is included
// because our API client (the Fluxer relay) translates the ids itself.
func keepsCustomEmoji(protocol string) bool {
	switch protocol {
	case "discord", "fluxer", apiProtocol:
		return true
	}
	return false
}

var channelMentionRE = regexp.MustCompile(`<#(\d+)>`)

// nameChannelMentions turns <#id> into #name, using Extra["discord_channels"] ({id: name}) when the source sent it.
func nameChannelMentions(text string, extra map[string][]any) string {
	names := map[string]any{}
	if extra != nil && len(extra["discord_channels"]) > 0 {
		if m, ok := extra["discord_channels"][0].(map[string]string); ok {
			for k, v := range m {
				names[k] = v
			}
		} else if m, ok := extra["discord_channels"][0].(map[string]any); ok {
			names = m
		}
	}
	return channelMentionRE.ReplaceAllStringFunc(text, func(m string) string {
		if n, ok := names[m[2:len(m)-1]].(string); ok && n != "" {
			return "#" + n
		}
		return "#channel"
	})
}

// stripCustomEmoji turns <:name:id> into :name: for destinations that can't render it.
func stripCustomEmoji(text string) string {
	return customEmojiRE.ReplaceAllString(text, ":$1:")
}

// emojiSprint converts :shortcode: to unicode everywhere except inside custom emoji markup,
// which would otherwise be mangled (<:smile:123> -> <😄123>).
func emojiSprint(text string) string {
	emoji.ReplacePadding = ""
	locs := customEmojiRE.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return emoji.Sprint(text)
	}
	out := make([]byte, 0, len(text))
	last := 0
	for _, l := range locs {
		out = append(out, emoji.Sprint(text[last:l[0]])...)
		out = append(out, text[l[0]:l[1]]...)
		last = l[1]
	}
	out = append(out, emoji.Sprint(text[last:])...)
	return string(out)
}
