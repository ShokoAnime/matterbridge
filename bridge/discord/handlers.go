package bdiscord

import (
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/matterbridge-org/matterbridge/bridge"
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

func (b *Bdiscord) messageDelete(s *discordgo.Session, m *discordgo.MessageDelete) { //nolint:unparam
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring messageDelete because it originates from a different guild")
		return
	}
	rmsg := config.Message{Account: b.Account, ID: m.ID, Event: config.EventMsgDelete, Text: config.EventMsgDelete, Extra: make(map[string][]any)}
	b.setChannel(&rmsg, m.ChannelID)

	b.Log.Debugf("<= Sending message from %s to gateway", b.Account)
	b.Log.Debugf("<= Message is %#v", rmsg)
	b.Remote <- rmsg
}

// TODO(qaisjp): if other bridges support bulk deletions, it could be fanned out centrally
func (b *Bdiscord) messageDeleteBulk(s *discordgo.Session, m *discordgo.MessageDeleteBulk) { //nolint:unparam
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring messageDeleteBulk because it originates from a different guild")
		return
	}
	for _, msgID := range m.Messages {
		rmsg := config.Message{
			Account: b.Account,
			ID:      msgID,
			Event:   config.EventMsgDelete,
			Text:    config.EventMsgDelete,
			Extra:   make(map[string][]any),
		}
		b.setChannel(&rmsg, m.ChannelID)

		b.Log.Debugf("<= Sending message from %s to gateway", b.Account)
		b.Log.Debugf("<= Message is %#v", rmsg)
		b.Remote <- rmsg
	}
}

func (b *Bdiscord) messageEvent(s *discordgo.Session, m *discordgo.Event) {
	b.Log.Debug(spew.Sdump(m.Struct))
}

func (b *Bdiscord) messageTyping(s *discordgo.Session, m *discordgo.TypingStart) {
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring messageTyping because it originates from a different guild")
		return
	}
	if !b.GetBool("ShowUserTyping") {
		return
	}

	// Ignore our own typing messages
	if m.UserID == b.userID {
		return
	}

	rmsg := config.Message{Account: b.Account, Event: config.EventUserTyping}
	rmsg.Channel = b.getChannelName(m.ChannelID)
	b.Remote <- rmsg
}

func (b *Bdiscord) messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) { //nolint:unparam
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring messageUpdate because it originates from a different guild")
		return
	}
	if b.GetBool("EditDisable") {
		return
	}
	// only when message is actually edited
	if m.EditedTimestamp != nil {
		// message must have been edited within EditMaxDays
		// (there is a discord glitch where old message edits get sent without user interaction)
		if b.GetInt("EditMaxDays") != 0 && time.Since(*m.EditedTimestamp) >= time.Duration(b.GetInt("EditMaxDays"))*24*time.Hour {
			return
		}

		b.Log.Debugf("Sending edit message")
		m.Content += b.GetString("EditSuffix")
		msg := &discordgo.MessageCreate{
			Message: m.Message,
		}
		b.relayMessage(s, msg, true)
	}
}

func (b *Bdiscord) handleQuote(s *discordgo.Session, m *discordgo.Message, msg string) string {
	if b.GetBool("QuoteDisable") {
		return msg
	}
	if m.MessageReference == nil {
		return msg
	}
	refMsgRef := m.MessageReference
	refMsg, err := s.ChannelMessage(refMsgRef.ChannelID, refMsgRef.MessageID)
	if err != nil {
		b.Log.Errorf("Error getting quoted message %s:%s: %s", refMsgRef.ChannelID, refMsgRef.MessageID, err)
		return msg
	}

	quoteMessage := refMsg.Content
	quoteNick := refMsg.Author.Username
	fromWebhook := m.WebhookID != ""
	if !fromWebhook && b.GetBool("UseDiscriminator") {
		quoteNick += "#" + refMsg.Author.Discriminator
	}

	format := b.GetString("quoteformat")
	if format == "" {
		format = "{MESSAGE} (re @{QUOTENICK}: {QUOTEMESSAGE})"
	}
	quoteMessagelength := len([]rune(quoteMessage))
	if b.GetInt("QuoteLengthLimit") != 0 && quoteMessagelength >= b.GetInt("QuoteLengthLimit") {
		runes := []rune(quoteMessage)
		quoteMessage = string(runes[0:b.GetInt("QuoteLengthLimit")])
		if quoteMessagelength > b.GetInt("QuoteLengthLimit") {
			quoteMessage += "..."
		}
	}
	format = strings.ReplaceAll(format, "{MESSAGE}", m.Content)
	format = strings.ReplaceAll(format, "{QUOTENICK}", quoteNick)
	format = strings.ReplaceAll(format, "{QUOTEMESSAGE}", quoteMessage)
	return format
}

func (b *Bdiscord) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) { //nolint:unparam
	b.relayMessage(s, m, false)
}

func (b *Bdiscord) relayMessage(s *discordgo.Session, m *discordgo.MessageCreate, isEdit bool) { //nolint:gocyclo,funlen
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring messageCreate because it originates from a different guild")
		return
	}
	var err error

	// only people's messages: not "thread created", thread starter copies, pins, joins, boosts...
	switch m.Type {
	case discordgo.MessageTypeDefault, discordgo.MessageTypeReply,
		discordgo.MessageTypeChatInputCommand, discordgo.MessageTypeContextMenuCommand:
	default:
		return
	}

	// not relay our own messages
	if m.Author.Username == b.nick {
		return
	}
	// if using webhooks, do not relay if it's ours
	if m.Author.Bot && b.transmitter.HasWebhook(m.Author.ID) {
		return
	}

	// UserID is the numeric Discord id (upstream behaviour; API clients rely on it to recognise people).
	rmsg := config.Message{Account: b.Account, Avatar: b.avatarURL(m.Author, m.Member, m.WebhookID != ""), UserID: m.Author.ID, ID: m.ID, IsEdit: isEdit, Extra: make(map[string][]any)}

	b.Log.Debugf("== Receiving event %#v", m.Message)

	if m.Content != "" {
		var channels map[string]string
		if b.GetBool("PreserveChannelMentions") {
			// keep <#id> for clients that map channels (names in Extra["discord_channels"] for everyone else)
			m.Message.Content, channels = b.protectChannelMentions(m.Message.Content)
		} else {
			m.Message.Content = b.replaceChannelMentions(m.Message.Content)
		}
		rmsg.Text, err = m.ContentWithMoreMentionsReplaced(b.c)
		if err != nil {
			b.Log.Errorf("ContentWithMoreMentionsReplaced failed: %s", err)
			rmsg.Text = m.ContentWithMentionsReplaced()
		}
		if channels != nil {
			rmsg.Text = restoreChannelMentions(rmsg.Text)
			rmsg.Extra["discord_channels"] = []any{channels}
		}
	}

	// set channel name (a thread's messages go to its parent channel's gateway, tagged with the thread)
	b.setChannel(&rmsg, m.ChannelID)

	fromWebhook := m.WebhookID != ""
	if !fromWebhook && !b.GetBool("UseUserName") {
		rmsg.Username = b.getNick(m.Author, m.GuildID)
	} else {
		rmsg.Username = m.Author.Username
		if !fromWebhook && b.GetBool("UseDiscriminator") {
			rmsg.Username += "#" + m.Author.Discriminator
		}
	}

	// raw embeds for clients that can show them (the API bridge passes Extra through), plus the text without the
	// embed summaries that ShowEmbeds adds below
	// Only bot-made ("rich") embeds count: link previews are rebuilt by every platform from the link itself.
	var richEmbeds []*discordgo.MessageEmbed
	for _, e := range m.Message.Embeds {
		if e != nil && (e.Type == discordgo.EmbedTypeRich || e.Type == "") {
			richEmbeds = append(richEmbeds, e)
		}
	}
	if len(richEmbeds) > 0 {
		embeds := make([]any, 0, len(richEmbeds))
		for _, e := range richEmbeds {
			embeds = append(embeds, e)
		}
		rmsg.Extra["discord_embeds"] = embeds
		rmsg.Extra["discord_text"] = []any{rmsg.Text}
	}

	// if we have embedded content add it to text
	if b.GetBool("ShowEmbeds") {
		for _, embed := range richEmbeds {
			rmsg.Text += b.handleEmbed(embed)
		}
	}

	// do we have a /me action
	var ok bool
	rmsg.Text, ok = b.replaceAction(rmsg.Text)
	if ok {
		rmsg.Event = config.EventUserAction
	}

	// Replace emotes (custom emoji markup can be kept for destinations that map it, see PreserveCustomEmoji)
	if !b.GetBool("PreserveCustomEmoji") {
		rmsg.Text = replaceEmotes(rmsg.Text)
	}

	// Handle Reply thread (API clients that make real replies get the text without the quote in Extra["discord_text"])
	if m.MessageReference != nil && len(rmsg.Extra["discord_text"]) == 0 {
		rmsg.Extra["discord_text"] = []any{rmsg.Text}
	}
	rmsg.Text = b.handleQuote(s, m.Message, rmsg.Text)

	// Add our parent id if it exists, and if it's not referring to a message in another channel
	if ref := m.MessageReference; ref != nil && ref.ChannelID == m.ChannelID {
		rmsg.ParentID = ref.MessageID
	}

	// no empty messages
	if rmsg.Text == "" && len(m.Attachments) == 0 {
		return
	}

	// if no attachments (or this is an edit: attachments can't change), send the message as-is
	if len(m.Attachments) == 0 || isEdit {
		if rmsg.Text == "" {
			return
		}
		b.Log.Debugf("<= Sending message from %s on %s to gateway", m.Author.Username, b.Account)
		b.Log.Debugf("<= Message is %#v", rmsg)

		b.Remote <- rmsg

		return
	}

	// We process attachments last, after all pre-processing is done
	// Perform the operations in the background, so we can process other
	// messages while we download the attachments.
	go func() {
		count := 0
		for _, attach := range m.Attachments {
			err := b.AddAttachmentFromURL(&rmsg, attach.Filename, attach.ID, "", attach.URL)
			if err != nil {
				b.Log.WithError(err).Warnf("Failed to download attachment %s", attach.Filename)
				// too big to carry along: link it instead, so the other side still gets the file
				var tooLarge *bridge.ErrFileTooLarge
				if errors.As(err, &tooLarge) {
					if rmsg.Text != "" {
						rmsg.Text += "\n"
					}
					rmsg.Text += attach.URL
					count += 1
				}
				continue
			}

			count += 1
		}

		if rmsg.Text == "" && count == 0 {
			b.Log.Warnf("Skipping message because there is no text and file uploads all failed")
			return
		}

		b.Log.Debugf("<= Sending message attachments from %s on %s to gateway", m.Author.Username, b.Account)
		b.Log.Debugf("<= Message is %#v", rmsg)

		b.Remote <- rmsg
	}()
}

func (b *Bdiscord) memberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring memberUpdate because it originates from a different guild")
		return
	}
	if m.Member == nil {
		b.Log.Warnf("Received member update with no member information: %#v", m)
	}

	b.membersMutex.Lock()
	defer b.membersMutex.Unlock()

	if currMember, ok := b.userMemberMap[m.Member.User.ID]; ok {
		b.Log.Debugf(
			"%s: memberupdate: user %s (nick %s) changes nick to %s",
			b.Account,
			m.Member.User.Username,
			b.userMemberMap[m.Member.User.ID].Nick,
			m.Member.Nick,
		)
		delete(b.nickMemberMap, currMember.User.Username)
		delete(b.nickMemberMap, currMember.Nick)
		delete(b.userMemberMap, m.Member.User.ID)
	}
	b.userMemberMap[m.Member.User.ID] = m.Member
	b.nickMemberMap[m.Member.User.Username] = m.Member
	if m.Member.Nick != "" {
		b.nickMemberMap[m.Member.Nick] = m.Member
	}
}

func (b *Bdiscord) memberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring memberAdd because it originates from a different guild")
		return
	}
	if b.GetBool("nosendjoinpart") {
		return
	}
	if m.Member == nil {
		b.Log.Warnf("Received member update with no member information: %#v", m)
		return
	}
	username := m.Member.User.Username
	if m.Member.Nick != "" {
		username = m.Member.Nick
	}

	rmsg := config.Message{
		Account:  b.Account,
		Event:    config.EventJoin,
		Username: "system",
		Text:     username + " joins",
	}
	b.Log.Debugf("<= Sending message from %s to gateway", b.Account)
	b.Log.Debugf("<= Message is %#v", rmsg)
	b.Remote <- rmsg
}

func (b *Bdiscord) memberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.GuildID != b.guildID {
		b.Log.Debugf("Ignoring memberRemove because it originates from a different guild")
		return
	}
	if b.GetBool("nosendjoinpart") {
		return
	}
	if m.Member == nil {
		b.Log.Warnf("Received member update with no member information: %#v", m)
		return
	}
	username := m.Member.User.Username
	if m.Member.Nick != "" {
		username = m.Member.Nick
	}

	rmsg := config.Message{
		Account:  b.Account,
		Event:    config.EventLeave,
		Username: "system",
		Text:     username + " leaves",
	}
	b.Log.Debugf("<= Sending message from %s to gateway", b.Account)
	b.Log.Debugf("<= Message is %#v", rmsg)
	b.Remote <- rmsg
}

func (b *Bdiscord) handleEmbed(embed *discordgo.MessageEmbed) string {
	var result string

	result = b.GetString("EmbedFormat")

	if result == "" {
		result = " embed: {TITLE} - {DESCRIPTION} - {URL}"
	}

	result = strings.ReplaceAll(result, "{URL}", embed.URL)
	result = strings.ReplaceAll(result, "{TITLE}", embed.Title)
	result = strings.ReplaceAll(result, "{DESCRIPTION}", embed.Description)
	result = strings.ReplaceAll(result, "{TIME}", embed.Timestamp)

	if embed.Footer != nil {
		result = strings.ReplaceAll(result, "{FOOTER}", embed.Footer.Text)
	} else {
		result = strings.ReplaceAll(result, "{FOOTER}", "")
	}

	if embed.Image != nil {
		result = strings.ReplaceAll(result, "{IMAGE}", embed.Image.URL)
	} else {
		result = strings.ReplaceAll(result, "{IMAGE}", "")
	}

	if embed.Thumbnail != nil {
		result = strings.ReplaceAll(result, "{THUMBNAIL}", embed.Thumbnail.URL)
	} else {
		result = strings.ReplaceAll(result, "{THUMBNAIL}", "")
	}

	if embed.Video != nil {
		result = strings.ReplaceAll(result, "{VIDEO}", embed.Video.URL)
	} else {
		result = strings.ReplaceAll(result, "{VIDEO}", "")
	}

	if embed.Provider != nil {
		result = strings.ReplaceAll(result, "{PROVIDER}", embed.Provider.URL)
	} else {
		result = strings.ReplaceAll(result, "{PROVIDER}", "")
	}

	if embed.Author != nil {
		result = strings.ReplaceAll(result, "{AUTHOR}", embed.Author.Name)
		result = strings.ReplaceAll(result, "{AUTHORURL}", embed.Author.URL)
	} else {
		result = strings.ReplaceAll(result, "{AUTHOR}", "")
		result = strings.ReplaceAll(result, "{AUTHORURL}", "")
	}

	if result != "" {
		result += "\n"
	}

	return result
}
