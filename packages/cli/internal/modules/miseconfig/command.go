package miseconfig

import "strings"

func shellQuote(s string) string   { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func windowsQuote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\"" }

// nativeCommand keeps normal script names readable and quotes user-defined names.
func nativeCommand(argv []string, windows bool) string {
	parts := make([]string, len(argv))
	for i, arg := range argv {
		if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_./:-", r))
		}) < 0 {
			parts[i] = arg
		} else if windows {
			parts[i] = windowsQuote(arg)
		} else {
			parts[i] = shellQuote(arg)
		}
	}
	return strings.Join(parts, " ")
}
