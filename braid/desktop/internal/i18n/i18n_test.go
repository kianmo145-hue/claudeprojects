package i18n

import "testing"

func TestEveryKeyHasBothLanguages(t *testing.T) {
	for k := range en {
		if _, ok := fa[k]; !ok {
			t.Errorf("fa is missing %q", k)
		}
	}
	for k := range fa {
		if _, ok := en[k]; !ok {
			t.Errorf("en is missing %q", k)
		}
	}
}

func TestPersianDigits(t *testing.T) {
	Set(FA)
	defer Set(EN)
	if got := T("total", "12.5", "3"); got != "مجموع  ↓۱۲٫۵  ↑۳ مگابیت" {
		t.Fatalf("%q", got)
	}
	if got := Digits("Wi-Fi.2 at 1.5"); got != "Wi-Fi.۲ at ۱٫۵" {
		t.Fatalf("%q", got)
	}
}
