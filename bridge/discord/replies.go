package bdiscord

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

// replyHeader is the first line of a webhook post that answers parentID: "-# ↪ <who> · [snippet](jump link)".
// <who> pings the parent's author when that is a real Discord user and the reply pinged (as a native reply would);
// after a silent reply, or for a bridged (webhook) author, it is their name. Returns "" if the parent can't be found.
func (b *Bdiscord) replyHeader(channelID, threadID, parentID string, ping bool) string {
	ch := channelID
	if threadID != "" {
		ch = threadID
	}
	parentID = strings.Split(parentID, ";")[0]
	pm, err := b.c.State.Message(ch, parentID)
	if err != nil {
		if pm, err = b.c.ChannelMessage(ch, parentID); err != nil {
			b.Log.Debugf("reply parent %s not found: %s", parentID, err)
			return ""
		}
	}
	who := ""
	if pm.Author != nil {
		switch {
		case pm.WebhookID == "" && !pm.Author.Bot && ping:
			who = "<@" + pm.Author.ID + ">"
		case pm.WebhookID == "" && !pm.Author.Bot: // a silent reply: the name, like Discord shows it, without a ping
			who = "**" + escapeMarkdown(b.getNick(pm.Author, b.guildID)) + "**"
		default:
			who = "**" + escapeMarkdown(pm.Author.Username) + "**"
		}
	}
	content := pm.Content
	if resolved, err := pm.ContentWithMoreMentionsReplaced(b.c); err == nil { // <@id> -> @nick, <#id> -> #channel...
		content = resolved
	}
	content = b.replaceChannelMentions(content)
	return fmt.Sprintf("-# ↪ %s · [%s](https://discord.com/channels/%s/%s/%s)",
		who, replySnippet(content, len(pm.Attachments) > 0, len(pm.Embeds) > 0), b.guildID, ch, parentID)
}

var (
	snippetURLRE   = regexp.MustCompile(`https?://[^\s<>]+`)
	snippetMarkRE  = regexp.MustCompile(`<a?(:\w+:)\d+>|<@[!&]?\d+>|<#\d+>|<t:\d+(?::\w)?>`)
	snippetSpaceRE = regexp.MustCompile(`\s+`)
)

// snippetText makes a quote safe for a masked link label: Discord won't render a label that contains a URL, and
// mention or emoji markup doesn't work inside one.
func snippetText(s string) string {
	s = snippetURLRE.ReplaceAllStringFunc(s, func(u string) string {
		if p, err := url.Parse(u); err == nil && p.Host != "" {
			return "🔗 " + strings.TrimPrefix(p.Host, "www.")
		}
		return "🔗"
	})
	s = snippetMarkRE.ReplaceAllStringFunc(s, func(m string) string {
		if sub := snippetMarkRE.FindStringSubmatch(m); sub[1] != "" {
			return sub[1] // custom emoji -> :name:
		}
		return "@…"
	})
	return strings.TrimSpace(snippetSpaceRE.ReplaceAllString(s, " "))
}

// replySnippet is the start of the parent's first line, safe inside a masked link.
func replySnippet(content string, hasFiles, hasEmbeds bool) string {
	line := snippetText(strings.SplitN(content, "\n", 2)[0])
	if line == "" {
		switch {
		case hasFiles:
			return "attachment"
		case hasEmbeds:
			return "embed"
		default:
			return "message"
		}
	}
	if r := []rune(line); len(r) > 80 {
		line = string(r[:80]) + "…"
	}
	return escapeMarkdown(line)
}

// Escapes markdown that would break or restyle the link label. "<" and ">" are left alone: escaping them shows the
// backslash.
var markdownEscaper = strings.NewReplacer("[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`", "~", "\\~", "|", "\\|")

func escapeMarkdown(s string) string { return markdownEscaper.Replace(s) }

// replyPing reports whether a bridged reply should ping the author it answers: Extra["reply_ping"] when the source
// said (a silent reply says false), else true, which is what a reply does by default.
func replyPing(msg *config.Message) bool {
	if v := msg.Extra["reply_ping"]; len(v) > 0 {
		if p, ok := v[0].(bool); ok {
			return p
		}
	}
	return true
}

// replyAllowedMentions is the bot's allowed mentions for a real reply: as configured, plus whether to ping the author.
func (b *Bdiscord) replyAllowedMentions(ping bool) *discordgo.MessageAllowedMentions {
	am := b.getAllowedMentions()
	if am == nil { // nothing configured: Discord's default, everything
		am = &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{
			discordgo.AllowedMentionTypeUsers, discordgo.AllowedMentionTypeRoles, discordgo.AllowedMentionTypeEveryone}}
	}
	am.RepliedUser = ping
	return am
}
