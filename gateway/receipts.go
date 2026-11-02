package gateway

import (
	"strings"

	"github.com/matterbridge-org/matterbridge/bridge/config"
)

// EventReceipt tells an API client where its message ended up: Extra["ids"] = [{account, protocol, id}].
// (API clients otherwise never learn the ids of their messages on the other bridges, e.g. to rewrite links.)
const EventReceipt = "api_receipt"

func (r *Router) sendReceipt(gw *Gateway, msg *config.Message, ids []*BrMsgID) {
	if msg.Protocol != apiProtocol || msg.Event != "" || len(ids) == 0 {
		return
	}
	src, ok := gw.Bridges[msg.Account]
	if !ok || src.Bridger == nil {
		return
	}
	var list []any
	for _, id := range ids {
		if id == nil || id.br == nil || id.br.Account == msg.Account {
			continue
		}
		list = append(list, map[string]any{"account": id.br.Account, "protocol": id.br.Protocol,
			"id": strings.TrimPrefix(id.ID, id.br.Protocol+" ")})
	}
	if len(list) == 0 {
		return
	}
	receipt := config.Message{Event: EventReceipt, ID: msg.ID, Gateway: gw.Name, Account: msg.Account,
		Channel: "api", Text: EventReceipt, Extra: map[string][]any{"ids": list}}
	if _, err := src.Send(receipt); err != nil {
		r.logger.Debugf("receipt for %s: %s", msg.ID, err)
	}
}
