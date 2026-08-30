package secrets

import "strings"

// MergeIntoEnviron combines the parent process environment with values loaded
// from a secrets backend. Default behaviour: shell vars win over the loaded
// map (matches dotenv-cli / node-dotenv defaults — least surprise for
// engineers used to those). override=true flips it: the loaded map wins,
// useful when the caller explicitly fetched values from a remote backend
// and wants them to take effect now.
//
// Used by `one run` (child env) and `one deploy` (build-time injection into
// each provider's CLI subprocess).
func MergeIntoEnviron(parent []string, vars map[string]string, override bool) []string {
	idx := make(map[string]int, len(parent))
	for i, kv := range parent {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		idx[kv[:eq]] = i
	}
	out := make([]string, len(parent))
	copy(out, parent)
	for k, v := range vars {
		if i, exists := idx[k]; exists {
			if override {
				out[i] = k + "=" + v
			}
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}
