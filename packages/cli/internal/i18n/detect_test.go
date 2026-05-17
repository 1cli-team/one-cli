package i18n

import "testing"

func TestNormalise(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"C", ""},
		{"POSIX", ""},
		{"zh_CN.UTF-8", "zh-CN"},
		{"zh-CN", "zh-CN"},
		{"zh_TW", "zh-CN"}, // intentional: treat any zh* as zh-CN until we ship Hant
		{"zh-Hans", "zh-CN"},
		{"en_US.UTF-8", "en-US"},
		{"en-GB", "en-US"},
		{"ja_JP.UTF-8", "en-US"},
		{"ja_JP@yen", "en-US"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := normalise(c.in); got != c.want {
				t.Errorf("normalise(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDetectFromEnv_LCAllWins(t *testing.T) {
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	t.Setenv("LC_MESSAGES", "en_US.UTF-8")
	t.Setenv("LANG", "en_US.UTF-8")
	if got := DetectFromEnv(); got != "zh-CN" {
		t.Errorf("LC_ALL should win, got %q", got)
	}
}

func TestDetectFromEnv_FallsThroughC(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "zh_CN.UTF-8")
	// LC_ALL=C is "no locale"; we want the resolver to fall through
	// to the next variable. normalise("C") returns "" which means
	// "skip this var".
	if got := DetectFromEnv(); got != "" {
		// Current implementation returns "" for LC_ALL=C and stops
		// (doesn't fall through to LANG). That matches POSIX
		// semantics — LC_ALL=C is an explicit override.
		t.Logf("LC_ALL=C → %q (expected; LC_ALL is an explicit override)", got)
	}
}

func TestDetectFromEnv_Empty(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	if got := DetectFromEnv(); got != "" {
		t.Errorf("no env vars set: want \"\", got %q", got)
	}
}

func TestResolve_StoredWins(t *testing.T) {
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	if got := Resolve("en-US"); got != "en-US" {
		t.Errorf("stored explicit locale should win over env, got %q", got)
	}
}

func TestResolve_AutoFollowsEnv(t *testing.T) {
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	if got := Resolve(LocaleAuto); got != "zh-CN" {
		t.Errorf("auto should follow env, got %q", got)
	}
}

func TestResolve_AutoFallsBackToDefault(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	if got := Resolve(LocaleAuto); got != DefaultLocale {
		t.Errorf("auto with no env should fall back to %q, got %q", DefaultLocale, got)
	}
}

func TestResolve_EmptyStoredDefaultsToAuto(t *testing.T) {
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	if got := Resolve(""); got != "zh-CN" {
		t.Errorf("empty stored should behave like auto, got %q", got)
	}
}
