// Package i18n holds the tray's texts in Persian and English.
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type Lang string

const (
	FA Lang = "fa"
	EN Lang = "en"
)

var cur atomic.Value // Lang

func init() { cur.Store(EN) }

// Set chooses the language; anything but "fa" means English.
func Set(l Lang) {
	if l != FA {
		l = EN
	}
	cur.Store(l)
}

func Current() Lang { return cur.Load().(Lang) }

// RTL reports whether the current language is written right to left.
func RTL() bool { return Current() == FA }

// T returns the text for key in the current language, formatted with args.
func T(key string, args ...any) string {
	m := en
	if Current() == FA {
		m = fa
	}
	s, ok := m[key]
	if !ok {
		s, ok = en[key]
		if !ok {
			s = key
		}
	}
	if len(args) > 0 {
		s = fmt.Sprintf(s, args...)
	}
	if Current() == FA {
		s = Digits(s)
	}
	return s
}

// Digits writes Western digits as Persian ones (and the decimal point as the Persian separator).
// Only digits that are part of numbers are touched: a dot between letters (a host name) stays.
func Digits(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune('۰' + (r - '0'))
		case r == '.' && i > 0 && i+1 < len(rs) && isDigit(rs[i-1]) && isDigit(rs[i+1]):
			b.WriteRune('٫')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Local is Digits in Persian and s unchanged in English.
func Local(s string) string {
	if Current() == FA {
		return Digits(s)
	}
	return s
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

var en = map[string]string{
	"app":            "Braid",
	"state.on":       "Merging",
	"state.off":      "Paused",
	"state.single":   "Paused: one connection",
	"state.starting": "Turning on…",
	"header":         "Braid · %s",
	"total":          "Total  ↓%s  ↑%s Mbps",
	"error":          "Error: %s",
	"row":            "%s   ↓%s ↑%s   %s",
	"row.flows":      " · %d flows",
	"row.nonet":      "  · no internet",
	"row.checking":   "  · checking…",
	"row.limit":      "  · limit %s",
	"row.v6":         " +IPv6",
	"limit.none":     "Unlimited",
	"limit.tip":      "Cap this link's speed (download and upload)",
	"phones":         "Phones (booster)",
	"phones.none":    "No phone found: turn on Booster in the phone app",
	"phone.online":   "connected",
	"phone.nodata":   "no mobile data",
	"phone.offline":  "not on this Wi-Fi",
	"phone.new":      "new, not paired",
	"phone.pair":     "Pair with this phone…",
	"phone.use":      "Use this phone",
	"phone.forget":   "Forget this phone",
	"phone.help":     "How does booster work?",
	"pair.title":     "Pair with %s",
	"pair.body":      "Check that the phone shows the same code:\n\n        %s\n\nIf it matches, tap Approve on the phone, then click OK here.\nClick Cancel if the codes are different.",
	"pair.ok":        "%s is paired. Its mobile data is now merged with this computer's connections.",
	"pair.fail":      "Pairing failed: %s",
	"pair.declined":  "The phone declined the pairing.",
	"toggle.on":      "Start merging",
	"toggle.off":     "Pause merging",
	"settings":       "Settings",
	"v6":             "Use all links (IPv6 only if every link has it)",
	"v6.tip":         "Ticked: apps are pushed to IPv4 so every link is used.",
	"autopause":      "Pause when there is only one connection",
	"signin":         "Start at sign-in",
	"lang":           "Language / زبان",
	"about":          "About Braid %s",
	"quit":           "Quit",
	"tip.total":      "Total ↓%s ↑%s",
	"tip.nonet":      " (no net)",
	"tip.unit":       " Mbps",
	"err.signin":     "start at sign-in: %s",
}

var fa = map[string]string{
	"app":            "Braid",
	"state.on":       "در حال ترکیب",
	"state.off":      "متوقف",
	"state.single":   "متوقف: فقط یک اتصال",
	"state.starting": "در حال روشن شدن…",
	"header":         "Braid · %s",
	"total":          "مجموع  ↓%s  ↑%s مگابیت",
	"error":          "خطا: %s",
	"row":            "%s   ↓%s ↑%s   %s",
	"row.flows":      " · %d اتصال",
	"row.nonet":      "  · بدون اینترنت",
	"row.checking":   "  · در حال بررسی…",
	"row.limit":      "  · سقف %s",
	"row.v6":         " +IPv6",
	"limit.none":     "بدون محدودیت",
	"limit.tip":      "محدود کردن سرعت این لینک (دانلود و آپلود)",
	"phones":         "گوشی‌ها (بوستر)",
	"phones.none":    "گوشی‌ای پیدا نشد: در اپ گوشی «بوستر» را روشن کنید",
	"phone.online":   "متصل",
	"phone.nodata":   "دیتای موبایل خاموش است",
	"phone.offline":  "روی این وای‌فای نیست",
	"phone.new":      "جدید، جفت نشده",
	"phone.pair":     "جفت شدن با این گوشی…",
	"phone.use":      "استفاده از این گوشی",
	"phone.forget":   "حذف این گوشی",
	"phone.help":     "بوستر چطور کار می‌کند؟",
	"pair.title":     "جفت شدن با %s",
	"pair.body":      "مطمئن شوید گوشی همین کد را نشان می‌دهد:\n\n        %s\n\nاگر یکسان است، روی گوشی «تأیید» را بزنید و بعد اینجا OK را بزنید.\nاگر کدها فرق دارند Cancel را بزنید.",
	"pair.ok":        "%s جفت شد. حالا دیتای موبایل آن با اتصال‌های این کامپیوتر ترکیب می‌شود.",
	"pair.fail":      "جفت شدن انجام نشد: %s",
	"pair.declined":  "گوشی درخواست را رد کرد.",
	"toggle.on":      "شروع ترکیب",
	"toggle.off":     "توقف ترکیب",
	"settings":       "تنظیمات",
	"v6":             "استفاده از همه لینک‌ها (IPv6 فقط اگر همه داشته باشند)",
	"v6.tip":         "تیک‌خورده: برنامه‌ها به IPv4 هدایت می‌شوند تا همه لینک‌ها استفاده شوند.",
	"autopause":      "توقف خودکار وقتی فقط یک اتصال هست",
	"signin":         "اجرا هنگام ورود به ویندوز",
	"lang":           "زبان / Language",
	"about":          "درباره Braid %s",
	"quit":           "خروج",
	"tip.total":      "مجموع ↓%s ↑%s",
	"tip.nonet":      " (قطع)",
	"tip.unit":       " مگابیت",
	"err.signin":     "اجرا هنگام ورود: %s",
}
