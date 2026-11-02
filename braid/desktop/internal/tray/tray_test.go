package tray

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"braid/internal/engine"
	. "braid/internal/i18n"
)

func TestShortenCountsUTF16AndKeepsRunesWhole(t *testing.T) {
	s := "braid ON  ↓12 ↑3.4 Mbit/s"
	if got := shorten(s, 127); got != s {
		t.Fatalf("short string changed: %q", got)
	}
	got := shorten(s, 16)
	if n := len(utf16.Encode([]rune(got))); n > 16 {
		t.Fatalf("%q is %d UTF-16 units, want <= 16", got, n)
	}
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "…") {
		t.Fatalf("bad cut: %q", got)
	}
}

func TestVersionLabel(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"0.3.3", "v0.3.3"},
		{"v0.3.3", "v0.3.3"}, // already prefixed: do not double it
		{"dev", "dev"},
		{"", "dev"}, // a build without -X main.version
	} {
		if got := versionLabel(c.in); got != c.want {
			t.Errorf("versionLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTrayTooltipFitsWindowsLimit(t *testing.T) {
	const mbit = 125_000
	mk := func(name string, down, up float64) engine.SlotStats {
		return engine.SlotStats{Name: name, Present: true, State: "up", LinkIn: down * mbit, LinkOut: up * mbit}
	}
	two := []engine.SlotStats{mk("Ethernet 3", 16, 0.2), mk("Ethernet 4", 79, 7)}
	tip := trayTooltip(stateWord(true), 95*mbit, 7.2*mbit, two)
	for _, want := range []string{"Merging", "Eth 3 ↓16 ↑0.2", "Eth 4 ↓79 ↑7.0", "Total ↓95 ↑7.2 Mbps"} {
		if utf16Len(tip) > tooltipMax || !strings.Contains(tip, want) {
			t.Fatalf("two adapters should fit with full detail (%q), got %d chars:\n%s", want, utf16Len(tip), tip)
		}
	}
	four := append(two, mk("Wi-Fi", 55, 3), mk("Ethernet 7", 12, 1))
	tip = trayTooltip(stateWord(false), 162*mbit, 11.2*mbit, four)
	if utf16Len(tip) > tooltipMax {
		t.Fatalf("tooltip is %d chars, Windows shows only %d:\n%s", utf16Len(tip), tooltipMax+1, tip)
	}
	for _, want := range []string{"Paused", "Eth 3 ↓", "Eth 4 ↓", "WiFi ↓", "Eth 7 ↓", "Total ↓162 ↑11"} {
		if !strings.Contains(tip, want) {
			t.Fatalf("every adapter keeps its download speed and the total is always shown; %q missing:\n%s", want, tip)
		}
	}
}

func TestLimitLabel(t *testing.T) {
	for in, want := range map[float64]string{1: "1", 2.5: "2.5", 100: "100"} {
		if got := limitLabel(in); got != want {
			t.Errorf("limitLabel(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestLimitTextUsesMbit(t *testing.T) {
	for in, want := range map[float64]string{1: "1 Mbit/s", 12.5: "12.5 Mbit/s", 1000: "1000 Mbit/s"} {
		if got := limitText(in); got != want {
			t.Errorf("limitText(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestPersianTooltipAndRow(t *testing.T) {
	Set(FA)
	defer Set(EN)
	const mbit = 125_000
	live := []engine.SlotStats{
		{Name: "Wi-Fi", Present: true, State: "up", LinkIn: 120 * mbit, LinkOut: 9 * mbit, Active: 12},
		{Name: "📱 Pixel", Present: true, State: "up", Remote: true, LinkIn: 80 * mbit, LinkOut: 3 * mbit},
	}
	tip := trayTooltip(stateWord(true), 200*mbit, 12*mbit, live)
	if utf16Len(tip) > tooltipMax || !strings.Contains(tip, "مجموع ↓۲۰۰") || !strings.Contains(tip, "📱 Pixel ↓۸۰") {
		t.Fatalf("tooltip:\n%s", tip)
	}
	row := rowText(live[0], 200*mbit, true)
	if !strings.Contains(row, "▰▰▰▱▱") || !strings.Contains(row, "۱۲ اتصال") {
		t.Fatalf("row: %q", row)
	}
}

func TestShareBar(t *testing.T) {
	for _, c := range []struct {
		part, total float64
		want        string
	}{{0, 0, "▱▱▱▱▱"}, {1, 1, "▰▰▰▰▰"}, {1, 2, "▰▰▰▱▱"}, {1, 10, "▰▱▱▱▱"}} {
		if got := shareBar(c.part, c.total); got != c.want {
			t.Errorf("shareBar(%v, %v) = %q, want %q", c.part, c.total, got, c.want)
		}
	}
}
