package link

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func get(m map[string]any, path ...string) any {
	var cur any = m
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func TestVLESSReality(t *testing.T) {
	p, err := Parse("vless://11111111-2222-3333-4444-555555555555@example.com:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.speedtest.net&fp=chrome&pbk=PUBKEY&sid=ab12&type=tcp#My%20Server")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "My Server" || p.Server != "example.com" || p.Port != 443 {
		t.Errorf("got %+v", p)
	}
	o := p.Outbound
	if o["flow"] != "xtls-rprx-vision" || get(o, "tls", "reality", "public_key") != "PUBKEY" || get(o, "tls", "server_name") != "www.speedtest.net" {
		t.Errorf("outbound %v", o)
	}
}

func TestVLESSWebSocketEarlyData(t *testing.T) {
	p, err := Parse("vless://id@1.2.3.4:8443?security=tls&type=ws&host=cdn.example.com&path=%2Fws%3Fed%3D2048&fp=firefox#ws")
	if err != nil {
		t.Fatal(err)
	}
	o := p.Outbound
	if get(o, "transport", "path") != "/ws" || get(o, "transport", "max_early_data") != 2048 {
		t.Errorf("transport %v", o["transport"])
	}
	if get(o, "tls", "server_name") != "cdn.example.com" || get(o, "tls", "utls", "fingerprint") != "firefox" {
		t.Errorf("tls %v", o["tls"])
	}
}

func TestVMess(t *testing.T) {
	j, _ := json.Marshal(map[string]any{"v": "2", "ps": "vm", "add": "v.example.com", "port": 443, "id": "uuid", "aid": "0", "net": "grpc", "path": "svc", "tls": "tls", "sni": "s.example.com"})
	p, err := Parse("vmess://" + base64.StdEncoding.EncodeToString(j))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "vm" || get(p.Outbound, "transport", "service_name") != "svc" || get(p.Outbound, "tls", "server_name") != "s.example.com" {
		t.Errorf("got %v", p.Outbound)
	}
}

func TestShadowsocksForms(t *testing.T) {
	sip002 := "ss://" + base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:pass")) + "@1.2.3.4:8388#a"
	legacy := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:p@ss@5.6.7.8:443")) + "#b"
	plain := "ss://2022-blake3-aes-128-gcm:a%2Bb@9.9.9.9:1234#c"
	for _, l := range []string{sip002, legacy, plain} {
		p, err := Parse(l)
		if err != nil {
			t.Fatalf("%s: %v", l, err)
		}
		if p.Outbound["method"] == "" || p.Outbound["password"] == "" {
			t.Errorf("%s: %v", l, p.Outbound)
		}
	}
	p, _ := Parse(legacy)
	if p.Outbound["password"] != "p@ss" || p.Server != "5.6.7.8" {
		t.Errorf("legacy: %v", p.Outbound)
	}
	p, _ = Parse(plain)
	if p.Outbound["password"] != "a+b" {
		t.Errorf("plain: %v", p.Outbound)
	}
}

func TestHysteria2PortHopping(t *testing.T) {
	p, err := Parse("hy2://secret@h.example.com:443,20000-30000/?obfs=salamander&obfs-password=xyz&sni=h.example.com#hy")
	if err != nil {
		t.Fatal(err)
	}
	if p.Port != 443 || get(p.Outbound, "obfs", "password") != "xyz" {
		t.Errorf("got %v", p.Outbound)
	}
	hops, _ := p.Outbound["server_ports"].([]string)
	if len(hops) != 1 || hops[0] != "20000:30000" {
		t.Errorf("hops %v", hops)
	}
}

func TestTUIC(t *testing.T) {
	p, err := Parse("tuic://uuid:pw@t.example.com:443?congestion_control=bbr&alpn=h3#t")
	if err != nil {
		t.Fatal(err)
	}
	if p.Outbound["password"] != "pw" || p.Outbound["congestion_control"] != "bbr" {
		t.Errorf("got %v", p.Outbound)
	}
}

func TestParseTextBase64Subscription(t *testing.T) {
	list := strings.Join([]string{
		"vless://id@a.example.com:443?security=tls#one",
		"trojan://pw@b.example.com:443#two",
		"vless://id@a.example.com:443?security=tls#one-renamed", // same server
		"wireguard://whatever",
		"garbage",
	}, "\n")
	ps, skipped := ParseText(base64.StdEncoding.EncodeToString([]byte(list)))
	if len(ps) != 2 {
		t.Errorf("got %d profiles", len(ps))
	}
	if len(skipped) != 2 {
		t.Errorf("skipped %v", skipped)
	}
}

func TestParseSingBoxJSON(t *testing.T) {
	ps, _ := ParseText(`{"outbounds":[{"type":"direct","tag":"d"},{"type":"vless","tag":"x","server":"s","server_port":443,"uuid":"u"}]}`)
	if len(ps) != 1 || ps[0].Name != "x" || ps[0].Outbound["tag"] != nil {
		t.Fatalf("got %+v", ps)
	}
	r, err := Restore(ps[0].Link)
	if err != nil || r.Outbound["uuid"] != "u" {
		t.Errorf("restore: %v %v", r, err)
	}
}
