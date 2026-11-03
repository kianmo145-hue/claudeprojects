package app

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"paydar/internal/link"
	"paydar/internal/store"
)

const userAgent = "Paydar/0.1 (sing-box)"

type fetched struct {
	profiles []*link.Profile
	skipped  []string
	title    string
	info     store.Subscription // traffic, expiry, interval
}

func subID(u string) string {
	h := sha1.Sum([]byte(strings.TrimSpace(u)))
	return hex.EncodeToString(h[:5])
}

// fetchSub downloads a subscription directly, then through the running proxy if that fails
// (subscription hosts are often filtered on the open internet).
func fetchSub(ctx context.Context, rawURL string, proxyPort int) (*fetched, error) {
	clients := []*http.Client{{Timeout: 20 * time.Second}}
	if proxyPort > 0 {
		pu, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
		clients = append(clients, &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu)}})
	}
	var last error
	for _, c := range clients {
		f, err := fetchWith(ctx, c, rawURL)
		if err == nil {
			return f, nil
		}
		last = err
	}
	return nil, last
}

func fetchWith(ctx context.Context, c *http.Client, rawURL string) (*fetched, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	ps, skipped := link.ParseText(string(body))
	if len(ps) == 0 {
		return nil, fmt.Errorf("no usable servers in the subscription (%d skipped)", len(skipped))
	}
	f := &fetched{profiles: ps, skipped: skipped}
	f.title = headerTitle(resp.Header.Get("Profile-Title"))
	if f.title == "" {
		f.title = contentDispositionName(resp.Header.Get("Content-Disposition"))
	}
	for _, kv := range strings.Split(resp.Header.Get("Subscription-Userinfo"), ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		switch strings.ToLower(k) {
		case "upload":
			f.info.Upload = n
		case "download":
			f.info.Download = n
		case "total":
			f.info.Total = n
		case "expire":
			f.info.Expire = n
		}
	}
	if h, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Profile-Update-Interval"))); err == nil && h > 0 {
		f.info.IntervalHours = h
	}
	return f, nil
}

func headerTitle(v string) string {
	v = strings.TrimSpace(v)
	if rest, ok := strings.CutPrefix(v, "base64:"); ok {
		if b, err := base64.StdEncoding.DecodeString(rest); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return v
}

func contentDispositionName(v string) string {
	for _, part := range strings.Split(v, ";") {
		k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(k, "filename") {
			return strings.Trim(val, `"`)
		}
	}
	return ""
}
