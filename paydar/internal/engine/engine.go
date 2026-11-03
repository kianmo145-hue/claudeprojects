// Package engine runs sing-box with a guard that keeps the connection on a working server.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"

	"paydar/internal/build"
	"paydar/internal/guard"
	"paydar/internal/link"
	"paydar/internal/logx"
)

// ProbeURLs are fetched through each server; the second is used when the first fails, so one
// blocked or slow test site does not mark a good server as down.
var ProbeURLs = []string{"https://www.gstatic.com/generate_204", "http://cp.cloudflare.com/generate_204"}

// directTargets answer quickly from almost anywhere; one of them working means the local
// network has internet.
var directTargets = []string{"1.1.1.1:443", "8.8.8.8:443", "9.9.9.9:443", "www.aparat.com:443"}

type Engine struct {
	ctx     context.Context
	cancel  context.CancelFunc
	box     *box.Box
	guard   *guard.Guard
	byTag   map[string]*link.Profile
	started time.Time
	wg      sync.WaitGroup
	closed  sync.Once
}

// Start builds and starts sing-box; the guard runs unless opts.Mode is ModeTest.
func Start(parent context.Context, profiles []*link.Profile, opts build.Options, gcfg guard.Config, logs *logx.Ring, notify func(guard.Event)) (*Engine, error) {
	content, err := build.Build(profiles, opts)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(include.Context(parent))
	options, err := json.UnmarshalExtendedContext[option.Options](ctx, content)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("config: %w", err)
	}
	bopts := box.Options{Options: options, Context: ctx}
	if logs != nil {
		bopts.PlatformLogWriter = logs
	}
	instance, err := box.New(bopts)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sing-box: %w", err)
	}
	if err := instance.Start(); err != nil {
		instance.Close()
		cancel()
		return nil, fmt.Errorf("start: %w", err)
	}
	e := &Engine{ctx: ctx, cancel: cancel, box: instance, byTag: map[string]*link.Profile{}, started: time.Now()}
	tags := make([]string, 0, len(profiles))
	for _, p := range profiles {
		tags = append(tags, build.Tag(p))
		e.byTag[build.Tag(p)] = p
	}
	current := build.Tag(profiles[0])
	if opts.Selected != "" {
		if _, ok := e.byTag[opts.Selected]; ok {
			current = opts.Selected
		}
	}
	e.guard = guard.New(gcfg, tags, current, e.probe, e.selectTag, e.directCheck, notify)
	if opts.Mode != build.ModeTest {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.guard.Run(ctx) }()
	}
	return e, nil
}

func (e *Engine) Guard() *guard.Guard { return e.guard }

func (e *Engine) Profile(tag string) *link.Profile { return e.byTag[tag] }

func (e *Engine) Since() time.Time { return e.started }

func (e *Engine) Close() {
	e.closed.Do(func() {
		e.cancel()
		done := make(chan struct{})
		go func() {
			e.wg.Wait()
			e.box.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(8 * time.Second):
		}
	})
}

func (e *Engine) outbound(tag string) (adapter.Outbound, error) {
	ob, ok := e.box.Outbound().Outbound(tag)
	if !ok {
		return nil, errors.New("no outbound " + tag)
	}
	return ob, nil
}

func (e *Engine) probe(ctx context.Context, tag string) (time.Duration, error) {
	ob, err := e.outbound(tag)
	if err != nil {
		return 0, err
	}
	var last error
	for i, u := range ProbeURLs {
		// Give the fallback URL its own share of the time left.
		pctx := ctx
		var cancel context.CancelFunc = func() {}
		if dl, ok := ctx.Deadline(); ok && i == 0 && len(ProbeURLs) > 1 {
			pctx, cancel = context.WithTimeout(ctx, time.Until(dl)*2/3)
		}
		ms, err := urltest.URLTest(pctx, u, ob)
		cancel()
		if err == nil {
			return time.Duration(ms) * time.Millisecond, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return 0, last
}

func (e *Engine) selectTag(tag string) error {
	ob, err := e.outbound(build.ProxyTag)
	if err != nil {
		return err
	}
	sel, ok := ob.(*group.Selector)
	if !ok {
		return errors.New("proxy is not a selector")
	}
	if !sel.SelectOutbound(tag) {
		return errors.New("unknown server " + tag)
	}
	return nil
}

func (e *Engine) directCheck(ctx context.Context) bool {
	ob, err := e.outbound(build.DirectTag)
	if err != nil {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	ok := make(chan bool, len(directTargets))
	for _, t := range directTargets {
		go func() {
			host, port, _ := net.SplitHostPort(t)
			c, err := ob.DialContext(ctx, "tcp", M.ParseSocksaddrHostPortStr(host, port))
			if err == nil {
				c.Close()
			}
			ok <- err == nil
		}()
	}
	for range directTargets {
		select {
		case v := <-ok:
			if v {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
	return false
}

// Test checks every profile without connecting (a temporary outbounds-only sing-box).
func Test(ctx context.Context, profiles []*link.Profile, directDNS []string, logs *logx.Ring) (map[string]guard.Health, error) {
	cfg := guard.DefaultConfig()
	cfg.Concurrency = 16
	e, err := Start(ctx, profiles, build.Options{Mode: build.ModeTest, DirectDNS: directDNS, LogLevel: "error"}, cfg, logs, nil)
	if err != nil {
		return nil, err
	}
	defer e.Close()
	e.guard.ProbeAll(ctx)
	out := map[string]guard.Health{}
	for _, h := range e.guard.Snapshot() {
		if p := e.byTag[h.Tag]; p != nil {
			out[p.ID] = h
		}
	}
	return out, nil
}

var _ io.Closer = (*box.Box)(nil)
