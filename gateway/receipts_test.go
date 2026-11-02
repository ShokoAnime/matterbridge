package gateway

import (
	"io"
	"net/http"
	"testing"

	"github.com/matterbridge-org/matterbridge/bridge"
	"github.com/matterbridge-org/matterbridge/bridge/config"
)

type recorder struct{ got []config.Message }

func (r *recorder) NewHttpRequest(string, string, io.Reader) (*http.Request, error) { return nil, nil }
func (r *recorder) NewHttpClient(string) (*http.Client, error)                      { return nil, nil }
func (r *recorder) SanitizeNick(*config.Message) error                              { return nil }

func (r *recorder) Send(m config.Message) (string, error) { r.got = append(r.got, m); return "", nil }
func (r *recorder) Connect() error                        { return nil }
func (r *recorder) JoinChannel(config.ChannelInfo) error  { return nil }
func (r *recorder) Disconnect() error                     { return nil }

func TestReceiptToAPISource(t *testing.T) {
	gw := newTestGateway("g", "api.fluxer", "discord.a")
	rec := &recorder{}
	api, disc := gw.Bridges["api.fluxer"], gw.Bridges["discord.a"]
	api.Protocol, disc.Protocol = "api", "discord"
	api.Bridger = rec
	r := &Router{}
	msg := &config.Message{Protocol: "api", Account: "api.fluxer", ID: "500"}
	r.sendReceipt(gw, msg, []*BrMsgID{{disc, "discord 900", "ID:1discord.a"}})
	if len(rec.got) != 1 || rec.got[0].Event != EventReceipt || rec.got[0].ID != "500" {
		t.Fatalf("receipt: %#v", rec.got)
	}
	id := rec.got[0].Extra["ids"][0].(map[string]any)
	if id["id"] != "900" || id["protocol"] != "discord" {
		t.Errorf("ids: %#v", id)
	}
	// not for messages from other bridges, nor for events
	r.sendReceipt(gw, &config.Message{Protocol: "discord", Account: "discord.a", ID: "1"}, []*BrMsgID{{api, "api x", "api"}})
	r.sendReceipt(gw, &config.Message{Protocol: "api", Account: "api.fluxer", ID: "2", Event: "msg_delete"}, []*BrMsgID{{disc, "discord 9", "x"}})
	if len(rec.got) != 1 {
		t.Errorf("unexpected receipts: %d", len(rec.got))
	}
	_ = bridge.Bridge{}
}
