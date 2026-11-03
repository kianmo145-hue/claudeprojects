package link

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ParseText parses whatever a user pastes or a subscription returns: one or more links (plain or
// base64-encoded), or a sing-box JSON config whose proxy outbounds are taken. Bad lines are
// reported in skipped and do not stop the rest.
func ParseText(text string) (profiles []*Profile, skipped []string) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\uFEFF"))
	if text == "" {
		return nil, nil
	}
	if strings.HasPrefix(text, "{") {
		if ps, err := parseSingBoxJSON(text); err == nil {
			return ps, nil
		}
	}
	if !strings.Contains(text, "://") {
		if dec, err := decodeB64(text); err == nil {
			text = string(dec)
		}
	}
	seen := map[string]bool{}
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		p, err := Parse(line)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", shorten(line), err))
			continue
		}
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		profiles = append(profiles, p)
	}
	return profiles, skipped
}

var proxyTypes = map[string]bool{
	"vless": true, "vmess": true, "trojan": true, "shadowsocks": true, "hysteria2": true,
	"hysteria": true, "tuic": true, "anytls": true, "shadowtls": true, "ssh": true, "socks": true, "http": true,
}

func parseSingBoxJSON(text string) ([]*Profile, error) {
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, err
	}
	var out []*Profile
	for _, ob := range cfg.Outbounds {
		typ, _ := ob["type"].(string)
		if !proxyTypes[typ] {
			continue
		}
		// detour chains and multiplex refer to other tags; drop the reference, keep the rest.
		delete(ob, "detour")
		name, _ := ob["tag"].(string)
		delete(ob, "tag")
		server, _ := ob["server"].(string)
		port := 0
		if f, ok := ob["server_port"].(float64); ok {
			port = int(f)
		}
		b, _ := json.Marshal(ob)
		h := sha1.Sum(b)
		p := &Profile{
			ID: hex.EncodeToString(h[:6]), Name: name, Type: typ, Server: server, Port: port,
			Link: "sing-box:" + string(b), Outbound: ob,
		}
		if p.Name == "" {
			p.Name = net.JoinHostPort(server, strconv.Itoa(port))
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no proxy outbounds")
	}
	return out, nil
}

// Restore rebuilds a profile from its stored Link (a share link or "sing-box:{json}").
func Restore(raw string) (*Profile, error) {
	if rest, ok := strings.CutPrefix(raw, "sing-box:"); ok {
		var ob map[string]any
		if err := json.Unmarshal([]byte(rest), &ob); err != nil {
			return nil, err
		}
		return &Profile{Outbound: ob, Link: raw}, nil
	}
	return Parse(raw)
}

func shorten(s string) string {
	if len(s) > 48 {
		return s[:48] + "…"
	}
	return s
}
