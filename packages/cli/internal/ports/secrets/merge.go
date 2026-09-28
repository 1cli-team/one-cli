package secrets

import (
	"runtime"
	"strings"
)

// MergeIntoEnviron combines shell variables with fetched values.
// When override is true, fetched values replace matching shell variables.
func MergeIntoEnviron(parent []string, vars map[string]string, override bool) []string {
	idx := make(map[string]int, len(parent))
	for i, kv := range parent {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		idx[environmentKey(kv[:eq])] = i
	}
	out := make([]string, len(parent))
	copy(out, parent)
	for k, v := range vars {
		if i, exists := idx[environmentKey(k)]; exists {
			if override {
				out[i] = k + "=" + v
			}
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func environmentKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
