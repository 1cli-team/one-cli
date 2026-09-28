package cli

import "strings"

// Expand only the top-level command token. Built-ins (including aliases) always
// win; arguments after -- and nested command names are never rewritten.
func expandTaskShorthand(args []string, known func(string) bool) []string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "help" || arg == "--help" || arg == "-h" || arg == "--version" || arg == "-v" {
			return args
		}
		if arg == "-o" || arg == "--output" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--output=") || strings.HasPrefix(arg, "-o") && len(arg) > 2 {
			continue
		}
		if arg == "" || strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "__") || known(arg) {
			return args
		}
		out := append([]string{}, args[:i]...)
		out = append(out, "run")
		return append(out, args[i:]...)
	}
	return args
}
