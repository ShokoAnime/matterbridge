package gateway

import (
	"os"
	"path/filepath"
	"testing"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/matterbridge-org/matterbridge/bridge"
	"github.com/sirupsen/logrus"
)

func newTestGateway(name string, accounts ...string) *Gateway {
	cache, _ := lru.New[string, []*BrMsgID](5000)
	gw := &Gateway{Name: name, Bridges: map[string]*bridge.Bridge{}, Messages: cache}
	for _, a := range accounts {
		gw.Bridges[a] = &bridge.Bridge{Account: a}
	}
	return gw
}

func TestMsgStoreSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ids.jsonl")
	log := logrus.NewEntry(logrus.New())

	gws := map[string]*Gateway{"g1": newTestGateway("g1", "discord.a", "api.b")}
	s := openMsgStore(path, log)
	s.load(gws)
	ids := []*BrMsgID{{gws["g1"].Bridges["api.b"], "api discord 1", "api"}}
	s.add("g1", "discord 1", ids)

	// "restart": fresh gateways and store, same file
	gws2 := map[string]*Gateway{"g1": newTestGateway("g1", "discord.a", "api.b")}
	s2 := openMsgStore(path, log)
	s2.load(gws2)
	got, ok := gws2["g1"].Messages.Get("discord 1")
	if !ok || len(got) != 1 || got[0].ID != "api discord 1" || got[0].br != gws2["g1"].Bridges["api.b"] {
		t.Fatalf("mapping not restored: %#v", got)
	}
}

func TestMsgStoreCompacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ids.jsonl")
	log := logrus.NewEntry(logrus.New())
	gws := map[string]*Gateway{"g1": newTestGateway("g1", "api.b")}
	s := openMsgStore(path, log)
	s.load(gws)
	ids := []*BrMsgID{{gws["g1"].Bridges["api.b"], "api x", "api"}}
	for i := 0; i < 3000; i++ { // same key rewritten: the file grows, the cache holds one entry
		s.add("g1", "discord 1", ids)
	}
	gws2 := map[string]*Gateway{"g1": newTestGateway("g1", "api.b")}
	openMsgStore(path, log).load(gws2)
	if s2 := openMsgStore(path, log); s2 != nil {
		s2.load(map[string]*Gateway{"g1": newTestGateway("g1", "api.b")})
		if s2.lines > 10 {
			t.Fatalf("file not compacted: %d lines", s2.lines)
		}
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("store file mode %v", fi.Mode().Perm())
	}
}
