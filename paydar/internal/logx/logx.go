// Package logx keeps the last log lines in memory for the UI.
package logx

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
)

type Line struct {
	Time  int64  `json:"time"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

type Ring struct {
	mu    sync.Mutex
	lines []Line
	max   int
	seq   int64
}

func NewRing(max int) *Ring { return &Ring{max: max} }

func (r *Ring) Add(level, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	r.mu.Lock()
	r.lines = append(r.lines, Line{Time: time.Now().UnixMilli(), Level: level, Text: text})
	if len(r.lines) > r.max {
		r.lines = r.lines[len(r.lines)-r.max:]
	}
	r.seq++
	r.mu.Unlock()
}

func (r *Ring) Infof(format string, a ...any)  { r.Add("info", fmt.Sprintf(format, a...)) }
func (r *Ring) Warnf(format string, a ...any)  { r.Add("warn", fmt.Sprintf(format, a...)) }
func (r *Ring) Errorf(format string, a ...any) { r.Add("error", fmt.Sprintf(format, a...)) }

func (r *Ring) Lines() []Line {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Line(nil), r.lines...)
}

// WriteMessage makes Ring a sing-box log.PlatformWriter.
func (r *Ring) WriteMessage(level log.Level, message string) {
	r.Add(log.FormatLevel(level), message)
}
