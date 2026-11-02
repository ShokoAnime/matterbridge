package bdiscord

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

// Reactions are mirrored with the bot account (webhooks can't react): the first reaction with an emoji on one side
// makes the bot add it on the other side, and the bot's reaction is removed once nobody has that emoji any more.

func reactionInfo(e discordgo.Emoji) map[string]any {
	return map[string]any{"name": e.Name, "id": e.ID, "animated": e.Animated}
}

func (b *Bdiscord) reactionEvent(event, channelID, messageID, userID string, emoji discordgo.Emoji, remaining int) {
	info := reactionInfo(emoji)
	text := emoji.Name
	if emoji.ID != "" {
		text = ":" + emoji.Name + ":"
	}
	if event == config.EventReactionRemove {
		info["remaining"] = remaining
	}
	rmsg := config.Message{Account: b.Account, Event: event, ID: messageID, UserID: userID, Text: text,
		Extra: map[string][]any{"reaction": {info}}}
	b.setChannel(&rmsg, channelID)
	b.Log.Debugf("<= Sending %s %s on %s to gateway", event, text, messageID)
	b.Remote <- rmsg
}

func (b *Bdiscord) messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) { //nolint:unparam
	if r.GuildID != b.guildID || r.UserID == b.userID || r.MessageReaction == nil {
		return
	}
	b.reactionEvent(config.EventReactionAdd, r.ChannelID, r.MessageID, r.UserID, r.Emoji, 0)
}

func (b *Bdiscord) messageReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) { //nolint:unparam
	if r.GuildID != b.guildID || r.UserID == b.userID || r.MessageReaction == nil {
		return
	}
	remaining := 0
	if m, err := b.c.ChannelMessage(r.ChannelID, r.MessageID); err == nil {
		for _, re := range m.Reactions {
			if re.Emoji != nil && re.Emoji.ID == r.Emoji.ID && (r.Emoji.ID != "" || re.Emoji.Name == r.Emoji.Name) {
				remaining = re.Count
				if re.Me {
					remaining--
				}
			}
		}
	}
	b.reactionEvent(config.EventReactionRemove, r.ChannelID, r.MessageID, r.UserID, r.Emoji, remaining)
}

// handleReaction mirrors a reaction from another bridge onto a message here, with the bot account.
func (b *Bdiscord) handleReaction(msg *config.Message, channelID string) error {
	if msg.ID == "" || msg.Extra == nil || len(msg.Extra["reaction"]) == 0 {
		return nil
	}
	info, ok := msg.Extra["reaction"][0].(map[string]any)
	if !ok {
		return nil
	}
	name, _ := info["name"].(string)
	id, _ := info["id"].(string)
	if name == "" {
		return nil
	}
	emoji := name
	if id != "" {
		emoji = name + ":" + id
	}
	msgID := strings.Split(msg.ID, ";")[0]
	if msg.Event == config.EventReactionAdd {
		return b.c.MessageReactionAdd(channelID, msgID, emoji)
	}
	if remaining, _ := info["remaining"].(float64); remaining > 0 {
		return nil // someone on the other side still has it
	}
	return b.c.MessageReactionRemove(channelID, msgID, emoji, "@me")
}
