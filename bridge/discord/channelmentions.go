package bdiscord

import (
	"regexp"
	"strings"
)

var (
	channelMentionIDRE = regexp.MustCompile(`<#(\d+)>`)
	protectedChannelRE = regexp.MustCompile("\x00ch(\\d+)\x00")
)

// protectChannelMentions hides <#id> from discordgo's mention replacement (which would turn it into #name) and
// returns the channels' names, for destinations that can't map the ids.
func (b *Bdiscord) protectChannelMentions(text string) (string, map[string]string) {
	names := map[string]string{}
	text = channelMentionIDRE.ReplaceAllStringFunc(text, func(m string) string {
		id := m[2 : len(m)-1]
		name := b.getChannelName(id)
		if name == "" || strings.HasPrefix(name, "ID:") {
			if ch, err := b.c.State.Channel(id); err == nil {
				name = ch.Name
			}
		}
		names[id] = strings.TrimPrefix(name, "ID:")
		return "\x00ch" + id + "\x00"
	})
	return text, names
}

func restoreChannelMentions(text string) string {
	return protectedChannelRE.ReplaceAllString(text, "<#$1>")
}
