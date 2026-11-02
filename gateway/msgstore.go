package gateway

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

// msgStore keeps the gateways' message-id mappings on disk, so edits, deletes and replies still find
// their counterpart after a restart. Enable it with `MessageIDStore="/path/file.jsonl"` under [general].
//
// The file is append-only JSON lines; on startup it is replayed into the in-memory caches and
// rewritten from them when it has grown well past what the caches hold.
type msgStore struct {
	sync.Mutex
	path  string
	f     *os.File
	lines int
	log   *logrus.Entry
}

type storedID struct {
	Account   string `json:"a"`
	ID        string `json:"i"`
	ChannelID string `json:"c"`
}

type storedEntry struct {
	Gateway string     `json:"g"`
	Key     string     `json:"k"`
	IDs     []storedID `json:"v"`
}

func openMsgStore(path string, log *logrus.Entry) *msgStore {
	if path == "" {
		return nil
	}
	return &msgStore{path: path, log: log}
}

// load replays the file into the gateways' caches, then reopens it for appending.
func (s *msgStore) load(gateways map[string]*Gateway) {
	if s == nil {
		return
	}
	s.Lock()
	defer s.Unlock()

	if f, err := os.Open(s.path); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			var e storedEntry
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			s.lines++
			gw, ok := gateways[e.Gateway]
			if !ok {
				continue
			}
			ids := make([]*BrMsgID, 0, len(e.IDs))
			for _, id := range e.IDs {
				if br, ok := gw.Bridges[id.Account]; ok {
					ids = append(ids, &BrMsgID{br, id.ID, id.ChannelID})
				}
			}
			gw.Messages.Add(e.Key, ids)
		}
		f.Close()
	} else if !os.IsNotExist(err) {
		s.log.Errorf("message id store: cannot read %s: %s", s.path, err)
	}

	kept := 0
	for _, gw := range gateways {
		kept += gw.Messages.Len()
	}
	if s.lines > 2*kept+1000 {
		s.compact(gateways)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		s.log.Errorf("message id store: cannot open %s: %s (ids will not survive a restart)", s.path, err)
		return
	}
	s.f = f
	s.log.Infof("message id store: loaded %d mappings from %s", kept, s.path)
}

// compact rewrites the file with only what the caches currently hold (oldest first).
func (s *msgStore) compact(gateways map[string]*Gateway) {
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		s.log.Errorf("message id store: compaction failed: %s", err)
		return
	}
	w := bufio.NewWriter(f)
	n := 0
	for name, gw := range gateways {
		for _, key := range gw.Messages.Keys() { // Keys() is oldest to newest
			ids, _ := gw.Messages.Peek(key)
			if b, err := json.Marshal(toStored(name, key, ids)); err == nil {
				w.Write(append(b, '\n'))
				n++
			}
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		s.log.Errorf("message id store: compaction failed: %s", err)
		return
	}
	f.Close()
	if err := os.Rename(tmp, s.path); err != nil {
		s.log.Errorf("message id store: compaction failed: %s", err)
		return
	}
	s.lines = n
}

func toStored(gateway, key string, ids []*BrMsgID) storedEntry {
	e := storedEntry{Gateway: gateway, Key: key}
	for _, id := range ids {
		if id == nil || id.br == nil {
			continue
		}
		e.IDs = append(e.IDs, storedID{id.br.Account, id.ID, id.ChannelID})
	}
	return e
}

// add records one mapping.
func (s *msgStore) add(gateway, key string, ids []*BrMsgID) {
	if s == nil {
		return
	}
	s.Lock()
	defer s.Unlock()
	if s.f == nil {
		return
	}
	b, err := json.Marshal(toStored(gateway, key, ids))
	if err != nil {
		return
	}
	if _, err := s.f.Write(append(b, '\n')); err != nil {
		s.log.Errorf("message id store: write failed: %s", err)
		return
	}
	s.lines++
}
