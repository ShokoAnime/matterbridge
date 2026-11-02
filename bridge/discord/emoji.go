package bdiscord

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// guildEmoji caches this guild's custom emoji by name, refreshed at most every few minutes
// (and on GUILD_EMOJIS_UPDATE via the session state, when it has them).
type guildEmoji struct {
	sync.Mutex
	byName  map[string]*discordgo.Emoji
	fetched time.Time
}

// shortcodeRE matches :name: candidates; replaceGuildEmoji skips those that are part of <:name:id> markup.
var shortcodeRE = regexp.MustCompile(`:(\w{2,32}):`)

func (b *Bdiscord) emojiByName() map[string]*discordgo.Emoji {
	b.emoji.Lock()
	defer b.emoji.Unlock()
	if b.emoji.byName != nil && time.Since(b.emoji.fetched) < 5*time.Minute {
		return b.emoji.byName
	}
	list, err := b.c.GuildEmojis(b.guildID)
	if err != nil {
		b.Log.Warnf("could not list guild emoji: %s", err)
		b.emoji.fetched = time.Now().Add(-4 * time.Minute) // retry in a minute
		return b.emoji.byName
	}
	m := make(map[string]*discordgo.Emoji, len(list))
	for _, e := range list {
		if e.Available || e.ID != "" {
			m[e.Name] = e
		}
	}
	b.emoji.byName, b.emoji.fetched = m, time.Now()
	return m
}

// replaceGuildEmoji turns :name: into <:name:id> when this server has a custom emoji of that exact name.
func (b *Bdiscord) replaceGuildEmoji(text string) string {
	if !shortcodeRE.MatchString(text) {
		return text
	}
	byName := b.emojiByName()
	if len(byName) == 0 {
		return text
	}
	return replaceShortcodes(text, func(name string) string {
		if e, ok := byName[name]; ok {
			return e.MessageFormat()
		}
		return ""
	})
}

// replaceShortcodes replaces each :name: (outside <:name:id> / <a:name:id> markup) with lookup(name),
// leaving it unchanged when lookup returns "".
func replaceShortcodes(text string, lookup func(string) string) string {
	var out strings.Builder
	last, pos := 0, 0
	for pos < len(text) {
		loc := shortcodeRE.FindStringSubmatchIndex(text[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		name := text[pos+loc[2] : pos+loc[3]]
		inMarkup := (start >= 1 && text[start-1] == '<') || (start >= 2 && text[start-1] == 'a' && text[start-2] == '<')
		if repl := lookup(name); !inMarkup && repl != "" {
			out.WriteString(text[last:start])
			out.WriteString(repl)
			last, pos = end, end
			continue
		}
		pos = start + 1 // the closing colon may open the next shortcode
	}
	out.WriteString(text[last:])
	return out.String()
}
