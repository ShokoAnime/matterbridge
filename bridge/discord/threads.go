package bdiscord

import (
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

// Discord threads are channels of their own. Their messages are routed through the parent channel's gateway and
// carry the thread in Extra["discord_thread"] ({id, name, parent_id, type, archived, locked}), so API clients can
// mirror threads. In the other direction a message whose Extra["discord_thread"] names a thread of the destination
// channel is posted into that thread.

// resolveThread returns the channel to route by (the parent, for a thread) and the thread's details if it is one.
func (b *Bdiscord) resolveThread(channelID string) (string, map[string]any) {
	ch, err := b.c.State.Channel(channelID)
	if err != nil {
		// after a restart the state only knows active threads; archived ones are fetched once
		ch, err = b.c.Channel(channelID)
		if err != nil {
			return channelID, nil
		}
		if ch.IsThread() {
			_ = b.c.State.ChannelAdd(ch)
		}
	}
	if !ch.IsThread() {
		return channelID, nil
	}
	info := map[string]any{"id": ch.ID, "name": ch.Name, "parent_id": ch.ParentID, "type": int(ch.Type)}
	if md := ch.ThreadMetadata; md != nil {
		info["archived"], info["locked"] = md.Archived, md.Locked
	}
	return ch.ParentID, info
}

// setChannel sets msg.Channel for a Discord channel id, resolving threads to their parent.
func (b *Bdiscord) setChannel(msg *config.Message, channelID string) {
	parent, thread := b.resolveThread(channelID)
	msg.Channel = b.getChannelName(parent)
	if thread != nil {
		if msg.Extra == nil {
			msg.Extra = make(map[string][]any)
		}
		msg.Extra["discord_thread"] = []any{thread}
	}
}

// threadFor returns the Discord thread a message should go to: the thread named in Extra["discord_thread"], if it
// exists and belongs to channelID; otherwise "" (post in the channel itself).
func (b *Bdiscord) threadFor(msg *config.Message, channelID string) string {
	if msg.Extra == nil || len(msg.Extra["discord_thread"]) == 0 {
		return ""
	}
	var id string
	switch t := msg.Extra["discord_thread"][0].(type) {
	case map[string]any:
		id, _ = t["id"].(string)
	case string:
		id = t
	}
	if id == "" {
		return ""
	}
	parent, thread := b.resolveThread(id)
	if thread == nil || parent != channelID {
		b.Log.Debugf("ignoring thread %s: not a thread of channel %s", id, channelID)
		return ""
	}
	return id
}

// threadParams is the ?thread_id= suffix for webhook calls into a thread.
func threadParam(threadID string) string {
	if threadID == "" {
		return ""
	}
	return "?thread_id=" + threadID
}
