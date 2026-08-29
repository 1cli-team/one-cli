package i18n

// detect.go resolves the locale the CLI should boot with, given:
//
//   1. The user's stored preference (preferences.Locale).
//   2. Their machine locale (LC_ALL > LC_MESSAGES > LANG).
//   3. A hard default ("en-US").
//
// Kept in i18n rather than preferences so the env-var parsing and
// BCP-47 normalisation live alongside the catalog keys they feed
// into — and so the preferences package stays free of i18n
// semantics it doesn't need.

import (
	"os"
	"strings"
)

// LocaleAuto is the sentinel value preferences uses for
// "follow the machine locale". Mirrored here as a typed constant so
// resolution code reads naturally:
//
//	resolved := i18n.Resolve(prefs.Locale)
const LocaleAuto = "auto"

// Resolve picks the locale to activate. Cases:
//
//   - storedLocale is one of "zh-CN" / "en-US" → that's it (user
//     explicitly picked, env vars are ignored).
//   - storedLocale is "" or "auto" → walk LC_ALL / LC_MESSAGES /
//     LANG. If we recognise one of them, use it. Else default.
//
// The returned value is always a locale the catalog can load (today
// "zh-CN" or "en-US"); never a raw env-var string.
func Resolve(storedLocale string) string {
	switch storedLocale {
	case "zh-CN", "en-US":
		return storedLocale
	}
	if detected := DetectFromEnv(); detected != "" {
		return detected
	}
	return DefaultLocale
}

// DetectFromEnv inspects the POSIX locale env vars in priority
// order (LC_ALL > LC_MESSAGES > LANG, matching the gettext spec)
// and maps the first non-empty one onto a supported catalog key.
// Returns "" if nothing recognisable is set, so the caller can
// chain on to a hard default.
//
// Supported patterns:
//
//	zh*       → "zh-CN"  (any zh variant, incl. zh_CN.UTF-8, zh-Hans)
//	C / POSIX → ""       (treated as "no preference" → caller default)
//	*         → "en-US"  (everything else falls back to English)
//
// We intentionally do NOT distinguish zh-TW / zh-Hant yet — until
// we ship a Traditional Chinese catalog, treating it the same as
// Simplified is closer to user intent than defaulting to English.
func DetectFromEnv() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			continue
		}
		return normalise(v)
	}
	return ""
}

// normalise turns a raw POSIX locale string ("zh_CN.UTF-8",
// "en-US", "C", "ja_JP@yen") into one of our catalog keys. Used by
// DetectFromEnv; exported for tests.
func normalise(raw string) string {
	// Strip codeset (".UTF-8") and modifier ("@yen") suffixes.
	v := raw
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	// POSIX "C" / "POSIX" means "no locale" — bubble up empty so
	// the caller's default wins.
	if v == "C" || v == "POSIX" || v == "" {
		return ""
	}
	// Both separators ("zh_CN" and "zh-CN") map onto the same tag.
	v = strings.ReplaceAll(v, "_", "-")
	v = strings.ToLower(v)
	if strings.HasPrefix(v, "zh") {
		return "zh-CN"
	}
	// Everything else routes to English. Once we ship more
	// catalogs we'll add cases above this fallthrough.
	return "en-US"
}
