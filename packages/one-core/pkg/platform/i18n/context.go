package i18n

import (
	"context"
	"fmt"
	"strings"
)

type localeContextKey struct{}
type Translator struct{ locale string }

// WithLocale supplies request language without changing the CLI process locale.
func WithLocale(ctx context.Context, language string) context.Context {
	locale := FallbackLocale
	for _, part := range strings.Split(language, ",") {
		part = strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if part == "zh" || strings.HasPrefix(part, "zh-") {
			locale = "zh-CN"
			break
		}
		if part == "en" || strings.HasPrefix(part, "en-") {
			break
		}
	}
	return context.WithValue(ctx, localeContextKey{}, locale)
}
func ForContext(ctx context.Context) Translator {
	locale, _ := ctx.Value(localeContextKey{}).(string)
	if locale == "" {
		locale = FallbackLocale
	}
	return Translator{locale: locale}
}
func (t Translator) T(key string) string {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	if value := catalogs[t.locale][key]; value != "" {
		return value
	}
	if value := catalogs[FallbackLocale][key]; value != "" {
		return value
	}
	return key
}
func (t Translator) Tf(key string, args ...any) string    { return fmt.Sprintf(t.T(key), args...) }
func (t Translator) Errorf(key string, args ...any) error { return fmt.Errorf(t.T(key), args...) }

func (t Translator) Locale() string { return t.locale }
