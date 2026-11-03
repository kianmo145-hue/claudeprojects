// Package link turns share links (vless://, vmess://, trojan://, ss://, hysteria2://, tuic://) and
// subscription bodies into sing-box outbound options.
package link

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Profile is one server config. Outbound is a sing-box outbound object without "tag".
type Profile struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Server   string         `json:"server"`
	Port     int            `json:"port"`
	Link     string         `json:"link"`
	Outbound map[string]any `json:"-"`
}

var ErrUnsupported = errors.New("unsupported")

// Parse parses a single share link.
func Parse(raw string) (*Profile, error) {
	raw = strings.TrimSpace(raw)
	scheme, _, ok := strings.Cut(raw, "://")
	if !ok {
		return nil, fmt.Errorf("not a link")
	}
	var (
		p   *Profile
		err error
	)
	switch strings.ToLower(scheme) {
	case "vless":
		p, err = parseURLStyle(raw, "vless")
	case "trojan":
		p, err = parseURLStyle(raw, "trojan")
	case "vmess":
		p, err = parseVMess(raw)
	case "ss":
		p, err = parseSS(raw)
	case "hysteria2", "hy2":
		p, err = parseHysteria2(raw)
	case "tuic":
		p, err = parseTUIC(raw)
	default:
		return nil, fmt.Errorf("%w scheme %q", ErrUnsupported, scheme)
	}
	if err != nil {
		return nil, err
	}
	p.Link = raw
	p.ID = idOf(raw)
	if p.Name == "" {
		p.Name = net.JoinHostPort(p.Server, strconv.Itoa(p.Port))
	}
	return p, nil
}

// idOf hashes the link without its #name, so renaming a server in a subscription keeps its history.
func idOf(raw string) string {
	if i := strings.LastIndex(raw, "#"); i > 0 && !strings.HasPrefix(raw, "vmess://") {
		raw = raw[:i]
	}
	h := sha1.Sum([]byte(raw))
	return hex.EncodeToString(h[:6])
}

func fragmentName(u *url.URL) string {
	if u.Fragment != "" {
		return strings.TrimSpace(u.Fragment)
	}
	return ""
}

func hostPort(u *url.URL) (string, int, error) {
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if host == "" || err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("bad server address %q", u.Host)
	}
	return host, port, nil
}

func truthy(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func splitList(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// streamParams are the v2ray-style transport and security settings shared by vless, trojan and vmess.
type streamParams struct {
	network, security                   string
	host, path, serviceName, headerType string
	sni, fp, alpn, pbk, sid             string
	insecure                            bool
}

func (s streamParams) tls(server string) (map[string]any, error) {
	switch s.security {
	case "", "none":
		return nil, nil
	case "tls", "xtls":
		t := map[string]any{"enabled": true}
		name := s.sni
		if name == "" {
			name = s.host
		}
		if name == "" && net.ParseIP(server) == nil {
			name = server
		}
		if name != "" {
			t["server_name"] = name
		}
		if s.insecure {
			t["insecure"] = true
		}
		if alpn := splitList(s.alpn); len(alpn) > 0 {
			t["alpn"] = alpn
		}
		if s.fp != "" && s.fp != "none" {
			t["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint(s.fp)}
		}
		return t, nil
	case "reality":
		if s.pbk == "" {
			return nil, errors.New("reality link without pbk")
		}
		t := map[string]any{
			"enabled":     true,
			"server_name": s.sni,
			"utls":        map[string]any{"enabled": true, "fingerprint": fingerprint(s.fp)},
			"reality":     map[string]any{"enabled": true, "public_key": s.pbk, "short_id": s.sid},
		}
		return t, nil
	}
	return nil, fmt.Errorf("%w security %q", ErrUnsupported, s.security)
}

func fingerprint(fp string) string {
	switch fp {
	case "chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized":
		return fp
	}
	return "chrome"
}

func (s streamParams) transport() (map[string]any, error) {
	switch s.network {
	case "", "tcp", "raw":
		if s.headerType == "http" {
			// v2ray's TCP "HTTP header" obfuscation has no sing-box equivalent.
			return nil, fmt.Errorf("%w tcp http header", ErrUnsupported)
		}
		return nil, nil
	case "ws", "websocket":
		t := map[string]any{"type": "ws"}
		path := s.path
		if path == "" {
			path = "/"
		}
		// "?ed=2048" in the path is Xray's early-data hint.
		if pu, err := url.Parse(path); err == nil {
			if ed, err := strconv.Atoi(pu.Query().Get("ed")); err == nil && ed > 0 {
				q := pu.Query()
				q.Del("ed")
				pu.RawQuery = q.Encode()
				path = pu.String()
				t["max_early_data"] = ed
				t["early_data_header_name"] = "Sec-WebSocket-Protocol"
			}
		}
		t["path"] = path
		if s.host != "" {
			t["headers"] = map[string]any{"Host": s.host}
		}
		return t, nil
	case "grpc":
		return map[string]any{"type": "grpc", "service_name": s.serviceName}, nil
	case "http", "h2":
		t := map[string]any{"type": "http"}
		if hosts := splitList(s.host); len(hosts) > 0 {
			t["host"] = hosts
		}
		if s.path != "" {
			t["path"] = s.path
		}
		return t, nil
	case "httpupgrade":
		t := map[string]any{"type": "httpupgrade"}
		if s.host != "" {
			t["host"] = s.host
		}
		if s.path != "" {
			t["path"] = s.path
		}
		return t, nil
	case "quic":
		return map[string]any{"type": "quic"}, nil
	}
	return nil, fmt.Errorf("%w transport %q", ErrUnsupported, s.network)
}

func (s streamParams) apply(out map[string]any, server string) error {
	tls, err := s.tls(server)
	if err != nil {
		return err
	}
	if tls != nil {
		out["tls"] = tls
	}
	tr, err := s.transport()
	if err != nil {
		return err
	}
	if tr != nil {
		out["transport"] = tr
	}
	return nil
}

func parseURLStyle(raw, kind string) (*Profile, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	server, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	cred := u.User.Username()
	if cred == "" {
		return nil, errors.New("missing user id / password")
	}
	sp := streamParams{
		network:     strings.ToLower(q.Get("type")),
		security:    strings.ToLower(q.Get("security")),
		host:        q.Get("host"),
		path:        q.Get("path"),
		serviceName: q.Get("serviceName"),
		headerType:  q.Get("headerType"),
		sni:         firstNonEmpty(q.Get("sni"), q.Get("peer")),
		fp:          q.Get("fp"),
		alpn:        q.Get("alpn"),
		pbk:         q.Get("pbk"),
		sid:         q.Get("sid"),
		insecure:    truthy(q.Get("allowInsecure")) || truthy(q.Get("insecure")),
	}
	out := map[string]any{"type": kind, "server": server, "server_port": port}
	switch kind {
	case "vless":
		out["uuid"] = cred
		if flow := q.Get("flow"); flow != "" {
			out["flow"] = flow
		}
		out["packet_encoding"] = "xudp"
	case "trojan":
		out["password"] = cred
		if sp.security == "" {
			sp.security = "tls" // trojan is TLS unless told otherwise
		}
	}
	if err := sp.apply(out, server); err != nil {
		return nil, err
	}
	return &Profile{Name: fragmentName(u), Type: kind, Server: server, Port: port, Outbound: out}, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// flexString accepts both "443" and 443 in vmess JSON.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexString(n.String())
	return nil
}

func parseVMess(raw string) (*Profile, error) {
	body, err := decodeB64(strings.TrimPrefix(raw[len("vmess://"):], "/"))
	if err != nil {
		return nil, fmt.Errorf("vmess: %w", err)
	}
	var v struct {
		PS, Add, ID, Scy, Net, Type, Host, Path, TLS, SNI, ALPN, FP flexString
		Port, Aid                                                   flexString
		Insecure                                                    flexString `json:"allowInsecure"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}
	port, _ := strconv.Atoi(string(v.Port))
	if v.Add == "" || port <= 0 || port > 65535 || v.ID == "" {
		return nil, errors.New("vmess: missing address, port or id")
	}
	aid, _ := strconv.Atoi(string(v.Aid))
	sec := string(v.Scy)
	if sec == "" {
		sec = "auto"
	}
	out := map[string]any{
		"type": "vmess", "server": string(v.Add), "server_port": port,
		"uuid": string(v.ID), "security": sec, "alter_id": aid, "packet_encoding": "xudp",
	}
	sp := streamParams{
		network: strings.ToLower(string(v.Net)), security: strings.ToLower(string(v.TLS)),
		host: string(v.Host), path: string(v.Path), serviceName: string(v.Path), headerType: string(v.Type),
		sni: string(v.SNI), fp: string(v.FP), alpn: string(v.ALPN), insecure: truthy(string(v.Insecure)),
	}
	if err := sp.apply(out, string(v.Add)); err != nil {
		return nil, err
	}
	return &Profile{Name: strings.TrimSpace(string(v.PS)), Type: "vmess", Server: string(v.Add), Port: port, Outbound: out}, nil
}

func parseSS(raw string) (*Profile, error) {
	rest := raw[len("ss://"):]
	name := ""
	if i := strings.LastIndex(rest, "#"); i >= 0 {
		name, _ = url.PathUnescape(rest[i+1:])
		rest = rest[:i]
	}
	query := ""
	if i := strings.Index(rest, "?"); i >= 0 {
		query = rest[i+1:]
		rest = rest[:i]
	}
	rest = strings.TrimSuffix(rest, "/")
	var userinfo, hostport string
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		userinfo, hostport = rest[:at], rest[at+1:]
		if dec, err := decodeB64(userinfo); err == nil && strings.Contains(string(dec), ":") {
			userinfo = string(dec)
		} else if un, err := url.PathUnescape(userinfo); err == nil {
			userinfo = un
		}
	} else {
		// Legacy: everything base64-encoded.
		dec, err := decodeB64(rest)
		if err != nil {
			return nil, fmt.Errorf("ss: %w", err)
		}
		at := strings.LastIndex(string(dec), "@")
		if at < 0 {
			return nil, errors.New("ss: bad link")
		}
		userinfo, hostport = string(dec[:at]), string(dec[at+1:])
	}
	method, password, ok := strings.Cut(userinfo, ":")
	if !ok || method == "" {
		return nil, errors.New("ss: missing method or password")
	}
	host, portS, err := net.SplitHostPort(hostport)
	port, perr := strconv.Atoi(portS)
	if err != nil || perr != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("ss: bad server address %q", hostport)
	}
	out := map[string]any{"type": "shadowsocks", "server": host, "server_port": port, "method": method, "password": password}
	if query != "" {
		q, _ := url.ParseQuery(query)
		if plugin := q.Get("plugin"); plugin != "" {
			pname, opts, _ := strings.Cut(plugin, ";")
			switch pname {
			case "obfs-local", "simple-obfs":
				pname = "obfs-local"
			case "v2ray-plugin":
			default:
				return nil, fmt.Errorf("%w ss plugin %q", ErrUnsupported, pname)
			}
			out["plugin"] = pname
			if opts != "" {
				out["plugin_opts"] = opts
			}
		}
	}
	return &Profile{Name: strings.TrimSpace(name), Type: "shadowsocks", Server: host, Port: port, Outbound: out}, nil
}

func parseHysteria2(raw string) (*Profile, error) {
	// Port hopping links look like host:443,2000-3000 which url.Parse rejects; take the first port.
	scheme, rest, _ := strings.Cut(raw, "://")
	var hops []string
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		end := len(rest)
		for _, sep := range []string{"/", "?", "#"} {
			if i := strings.Index(rest[at:], sep); i >= 0 && at+i < end {
				end = at + i
			}
		}
		hp := rest[at+1 : end]
		if c := strings.LastIndex(hp, ":"); c >= 0 && strings.ContainsAny(hp[c+1:], ",-") {
			ports := strings.Split(hp[c+1:], ",")
			for _, p := range ports[1:] {
				hops = append(hops, strings.ReplaceAll(p, "-", ":"))
			}
			if strings.Contains(ports[0], "-") {
				hops = append(hops, strings.ReplaceAll(ports[0], "-", ":"))
				ports[0], _, _ = strings.Cut(ports[0], "-")
			}
			rest = rest[:at+1] + hp[:c+1] + ports[0] + rest[end:]
		}
	}
	u, err := url.Parse(scheme + "://" + rest)
	if err != nil {
		return nil, err
	}
	server, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	password := u.User.Username()
	if pw, ok := u.User.Password(); ok {
		password += ":" + pw
	}
	out := map[string]any{"type": "hysteria2", "server": server, "server_port": port, "password": password}
	if mp := q.Get("mport"); mp != "" {
		for _, p := range splitList(mp) {
			hops = append(hops, strings.ReplaceAll(p, "-", ":"))
		}
	}
	if len(hops) > 0 {
		out["server_ports"] = hops
	}
	if o := q.Get("obfs"); o != "" && o != "none" {
		out["obfs"] = map[string]any{"type": o, "password": q.Get("obfs-password")}
	}
	tls := map[string]any{"enabled": true}
	if sni := q.Get("sni"); sni != "" {
		tls["server_name"] = sni
	} else if net.ParseIP(server) == nil {
		tls["server_name"] = server
	}
	if truthy(q.Get("insecure")) {
		tls["insecure"] = true
	}
	if alpn := splitList(q.Get("alpn")); len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	out["tls"] = tls
	return &Profile{Name: fragmentName(u), Type: "hysteria2", Server: server, Port: port, Outbound: out}, nil
}

func parseTUIC(raw string) (*Profile, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	server, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	pw, _ := u.User.Password()
	out := map[string]any{"type": "tuic", "server": server, "server_port": port, "uuid": u.User.Username(), "password": pw}
	if cc := q.Get("congestion_control"); cc != "" {
		out["congestion_control"] = cc
	}
	if m := q.Get("udp_relay_mode"); m != "" {
		out["udp_relay_mode"] = m
	}
	tls := map[string]any{"enabled": true}
	if sni := q.Get("sni"); sni != "" {
		tls["server_name"] = sni
	} else if net.ParseIP(server) == nil {
		tls["server_name"] = server
	}
	if truthy(q.Get("allow_insecure")) || truthy(q.Get("insecure")) {
		tls["insecure"] = true
	}
	alpn := splitList(q.Get("alpn"))
	if len(alpn) == 0 {
		alpn = []string{"h3"}
	}
	tls["alpn"] = alpn
	out["tls"] = tls
	return &Profile{Name: fragmentName(u), Type: "tuic", Server: server, Port: port, Outbound: out}, nil
}

func decodeB64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("not base64")
}
