package bdiscord

import (
	"fmt"
	"strings"
)

// replyHeader is the first line of a webhook post that answers parentID: "-# ↪ <who> · [snippet](jump link)".
// <who> pings the parent's author when that is a real Discord user (as a native reply would); a bridged (webhook)
// author is shown by name. Returns "" if the parent can't be found.
func (b *Bdiscord) replyHeader(channelID, threadID, parentID string) string {
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
		if pm.WebhookID == "" && !pm.Author.Bot {
			who = "<@" + pm.Author.ID + ">"
		} else {
			who = "**" + escapeMarkdown(pm.Author.Username) + "**"
		}
	}
	return fmt.Sprintf("-# ↪ %s · [%s](https://discord.com/channels/%s/%s/%s)",
		who, replySnippet(pm.Content, len(pm.Attachments) > 0, len(pm.Embeds) > 0), b.guildID, ch, parentID)
}

// replySnippet is the start of the parent's first line, safe inside a masked link.
func replySnippet(content string, hasFiles, hasEmbeds bool) string {
	line := strings.TrimSpace(strings.SplitN(content, "\n", 2)[0])
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

var markdownEscaper = strings.NewReplacer("[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`", "~", "\\~", "|", "\\|", ">", "\\>")

func escapeMarkdown(s string) string { return markdownEscaper.Replace(s) }
