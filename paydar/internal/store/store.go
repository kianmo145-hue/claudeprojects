// Package store saves servers, subscriptions and settings as one JSON file.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Server struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Server string `json:"server"`
	Port   int    `json:"port"`
	Link   string `json:"link"`
	Sub    string `json:"sub,omitempty"`
	// Last test result, kept so the list shows something useful right after start.
	LastRTT    int   `json:"lastRtt,omitempty"`
	LastTested int64 `json:"lastTested,omitempty"`
	LastOK     bool  `json:"lastOkResult,omitempty"`
}

type Subscription struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	UpdatedAt     int64  `json:"updatedAt,omitempty"`
	Upload        int64  `json:"upload,omitempty"`
	Download      int64  `json:"download,omitempty"`
	Total         int64  `json:"total,omitempty"`
	Expire        int64  `json:"expire,omitempty"`
	IntervalHours int    `json:"intervalHours,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	Count         int    `json:"count"`
}

type Settings struct {
	// Mode on Windows: "tun" (whole system) or "proxy" (system proxy). Android ignores it.
	Mode         string `json:"mode"`
	AutoSwitch   bool   `json:"autoSwitch"`
	PreferFaster bool   `json:"preferFaster"`
	BypassIran   bool   `json:"bypassIran"`
	IPv6         bool   `json:"ipv6"`
	RemoteDNS    string `json:"remoteDns"`
	MixedPort    int    `json:"mixedPort"`
	Lang         string `json:"lang"`
	AutoConnect  bool   `json:"autoConnect"`
	// SubRefreshHours: 0 = follow the subscription's own interval (default 12h).
	SubRefreshHours int `json:"subRefreshHours"`
}

func DefaultSettings() Settings {
	return Settings{
		Mode: "tun", AutoSwitch: true, PreferFaster: true, BypassIran: true,
		RemoteDNS: "https://1.1.1.1/dns-query", MixedPort: 12334, Lang: "fa",
	}
}

type State struct {
	Servers  []Server       `json:"servers"`
	Subs     []Subscription `json:"subs"`
	Settings Settings       `json:"settings"`
	// Selected: the user's pick. LastGood: where the guard last had a working connection.
	Selected string `json:"selected,omitempty"`
	LastGood string `json:"lastGood,omitempty"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func Open(dir string) (*Store, State, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, State{}, err
	}
	s := &Store{path: filepath.Join(dir, "paydar.json")}
	st := State{Settings: DefaultSettings()}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, st, nil
	}
	if err != nil {
		return nil, st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		// A broken file must not lock the user out: keep it aside and start clean.
		os.Rename(s.path, s.path+".broken")
		return s, State{Settings: DefaultSettings()}, nil
	}
	if st.Settings.MixedPort == 0 {
		st.Settings.MixedPort = DefaultSettings().MixedPort
	}
	return s, st, nil
}

// Save writes atomically (temp file + rename).
func (s *Store) Save(st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
