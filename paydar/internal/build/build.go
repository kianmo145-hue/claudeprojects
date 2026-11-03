// Package build writes the sing-box configuration for a list of servers.
package build

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"paydar/internal/link"
)

const (
	ProxyTag  = "proxy"
	DirectTag = "direct"
)

// Mode is how traffic reaches sing-box.
type Mode string

const (
	// ModeTUN: sing-box's own TUN adapter captures the whole system (Windows, needs admin).
	ModeTUN Mode = "tun"
	// ModeSystemProxy: a local proxy set as the Windows system proxy.
	ModeSystemProxy Mode = "proxy"
	// ModeLocal: only a local proxy; Android feeds it from its VpnService TUN.
	ModeLocal Mode = "local"
	// ModeTest: outbounds only, for testing servers while disconnected.
	ModeTest Mode = "test"
)

type Options struct {
	Mode      Mode
	MixedPort int
	// DirectDNS resolves server addresses and bypassed domains: "local" or a list of IPs.
	DirectDNS []string
	// RemoteDNS answers apps' queries through the proxy, e.g. https://1.1.1.1/dns-query.
	RemoteDNS  string
	BypassIran bool
	IPv6       bool
	Selected   string
	LogLevel   string
}

func Tag(p *link.Profile) string { return "p-" + p.ID }

// Build returns the sing-box JSON config.
func Build(profiles []*link.Profile, o Options) ([]byte, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no servers")
	}
	var outbounds []any
	tags := make([]string, 0, len(profiles))
	for _, p := range profiles {
		ob := make(map[string]any, len(p.Outbound)+1)
		for k, v := range p.Outbound {
			ob[k] = v
		}
		ob["tag"] = Tag(p)
		outbounds = append(outbounds, ob)
		tags = append(tags, Tag(p))
	}
	selected := o.Selected
	if !contains(tags, selected) {
		selected = tags[0]
	}
	outbounds = append(outbounds,
		map[string]any{
			"type": "selector", "tag": ProxyTag, "outbounds": tags, "default": selected,
			// A switch closes connections still on the old server, so apps reconnect at once
			// instead of hanging on a dead tunnel.
			"interrupt_exist_connections": true,
		},
		map[string]any{"type": "direct", "tag": DirectTag},
	)

	cfg := map[string]any{
		"log":       map[string]any{"level": firstNonEmpty(o.LogLevel, "warn"), "timestamp": true},
		"outbounds": outbounds,
	}
	if o.Mode == ModeTest {
		cfg["dns"] = map[string]any{"servers": []any{directDNS(o.DirectDNS)}, "final": "dns-direct"}
		cfg["route"] = map[string]any{"final": DirectTag, "default_domain_resolver": map[string]any{"server": "dns-direct"}}
		return json.MarshalIndent(cfg, "", "  ")
	}

	remote, err := remoteDNS(firstNonEmpty(o.RemoteDNS, "https://1.1.1.1/dns-query"))
	if err != nil {
		return nil, err
	}
	dnsRules := []any{}
	routeRules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
		map[string]any{"port": 53, "action": "hijack-dns"},
		map[string]any{"ip_is_private": true, "outbound": DirectTag},
	}
	if o.BypassIran {
		dnsRules = append(dnsRules, map[string]any{"domain_suffix": []string{"ir"}, "server": "dns-direct"})
		routeRules = append(routeRules, map[string]any{"domain_suffix": []string{"ir"}, "outbound": DirectTag})
	}
	strategy := "ipv4_only"
	if o.IPv6 {
		strategy = "prefer_ipv4"
	}
	cfg["dns"] = map[string]any{
		"servers":  []any{remote, directDNS(o.DirectDNS)},
		"rules":    dnsRules,
		"final":    "dns-remote",
		"strategy": strategy,
	}
	route := map[string]any{
		"rules":                   routeRules,
		"final":                   ProxyTag,
		"default_domain_resolver": map[string]any{"server": "dns-direct", "strategy": strategy},
	}
	inbounds := []any{}
	mixed := map[string]any{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": o.MixedPort}
	switch o.Mode {
	case ModeTUN:
		addr := []string{"172.19.0.1/30"}
		if o.IPv6 {
			addr = append(addr, "fdfe:dcba:9876::1/126")
		}
		inbounds = append(inbounds, map[string]any{
			"type": "tun", "tag": "tun-in", "interface_name": "Paydar",
			"address": addr, "mtu": 9000, "auto_route": true, "strict_route": true, "stack": "mixed",
		})
		route["auto_detect_interface"] = true
	case ModeSystemProxy:
		mixed["set_system_proxy"] = true
		route["auto_detect_interface"] = true
	case ModeLocal:
	default:
		return nil, fmt.Errorf("unknown mode %q", o.Mode)
	}
	if o.MixedPort > 0 {
		inbounds = append(inbounds, mixed)
	}
	cfg["inbounds"] = inbounds
	cfg["route"] = route
	return json.MarshalIndent(cfg, "", "  ")
}

func directDNS(servers []string) map[string]any {
	for _, s := range servers {
		if s != "" && s != "local" {
			return map[string]any{"type": "udp", "tag": "dns-direct", "server": s}
		}
	}
	return map[string]any{"type": "local", "tag": "dns-direct"}
}

// remoteDNS accepts https://, tls://, tcp://, udp:// or a bare IP.
func remoteDNS(s string) (map[string]any, error) {
	if !strings.Contains(s, "://") {
		s = "udp://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("bad DNS server %q", s)
	}
	d := map[string]any{"tag": "dns-remote", "server": u.Hostname(), "detour": ProxyTag}
	switch u.Scheme {
	case "https":
		d["type"] = "https"
		if u.Path != "" && u.Path != "/dns-query" {
			d["path"] = u.Path
		}
	case "tls", "tcp", "udp":
		d["type"] = u.Scheme
	default:
		return nil, fmt.Errorf("bad DNS server %q", s)
	}
	if u.Port() != "" {
		var port int
		fmt.Sscanf(u.Port(), "%d", &port)
		d["server_port"] = port
	}
	return d, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
