package gateway

import (
	"strings"

	"github.com/matterbridge-org/matterbridge/bridge"
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

// reactionSupport lists the protocols that mirror reactions (with their bot account).
var reactionSupport = map[string]struct{}{"discord": {}, apiProtocol: {}}

func isReaction(event string) bool {
	return event == config.EventReactionAdd || event == config.EventReactionRemove
}

// reactionTargetID finds the destination's copy of the message that was reacted to. People usually react to a
// bridged copy, so besides the direct mapping (original -> copies) it also resolves copy -> original -> copy.
func (gw *Gateway) reactionTargetID(rmsg *config.Message, dest *bridge.Bridge, channel *config.ChannelInfo) string {
	if id := gw.getDestMsgID(rmsg.Protocol+" "+rmsg.ID, dest, channel); id != "" {
		return id
	}
	canonical := gw.FindCanonicalMsgID(rmsg.Protocol, rmsg.ID)
	if canonical == "" {
		return ""
	}
	if strings.HasPrefix(canonical, dest.Protocol+" ") { // the original is on the destination itself
		return strings.TrimPrefix(canonical, dest.Protocol+" ")
	}
	return gw.getDestMsgID(canonical, dest, channel)
}
