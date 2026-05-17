package i18n

import (
	"sort"
	"testing"
)

func TestT_FallsBackToKey(t *testing.T) {
	_ = Init(DefaultLocale)
	// A key we never added returns itself.
	if got := T("does.not.exist"); got != "does.not.exist" {
		t.Errorf("unknown key: want fallthrough to key, got %q", got)
	}
}

func TestT_TranslatesKnownKey(t *testing.T) {
	if err := Init("en-US"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	got := T("root.short")
	if got == "" || got == "root.short" {
		t.Errorf("root.short en-US did not translate: %q", got)
	}
}

func TestT_LocaleSwap(t *testing.T) {
	_ = Init("en-US")
	en := T("create.short")
	_ = Init("zh-CN")
	zh := T("create.short")
	if en == zh {
		t.Errorf("locale swap had no effect: en=%q zh=%q", en, zh)
	}
}

func TestT_FallbackToEnglishOnUnknownLocale(t *testing.T) {
	_ = Init("klingon")
	if Active() != DefaultLocale {
		t.Errorf("unknown locale should fall back to %q, got %q", DefaultLocale, Active())
	}
}

func TestAvailableLocales_StableSet(t *testing.T) {
	ensureLoaded()
	got := AvailableLocales()
	sort.Strings(got)
	want := []string{"en-US", "zh-CN"}
	if len(got) != len(want) {
		t.Fatalf("AvailableLocales: want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("AvailableLocales[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

func TestT_CatalogParity(t *testing.T) {
	// Every key in en-US must also exist in zh-CN. We translate
	// incrementally per the project plan, but missing zh-CN
	// translations would silently fall back to English which is
	// not what we want for first-class locales.
	ensureLoaded()
	for k, v := range catalogs[FallbackLocale] {
		zh, ok := catalogs["zh-CN"]
		if !ok {
			t.Fatal("zh-CN catalog not loaded")
		}
		got, present := zh[k]
		if !present || got == "" {
			t.Errorf("zh-CN missing translation for key %q (en-US has %q)", k, v)
		}
	}
}
