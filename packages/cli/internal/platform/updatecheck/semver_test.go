package updatecheck

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		// strictly greater triples
		{"v0.9.0", "v0.8.0", true},
		{"v0.8.1", "v0.8.0", true},
		{"v1.0.0", "v0.99.99", true},
		{"v10.0.0", "v9.0.0", true},

		// equal
		{"v0.8.0", "v0.8.0", false},

		// strictly older
		{"v0.7.0", "v0.8.0", false},
		{"v0.8.0", "v0.8.1", false},

		// pre-release suffixes are stripped — same triple = not newer
		{"v0.9.0-rc1", "v0.9.0", false},
		{"v0.9.0", "v0.9.0-rc1", false},
		// but a higher base still wins despite a suffix
		{"v0.9.0-rc1", "v0.8.0", true},

		// missing segments default to 0 (matches install.sh)
		{"v1", "v1.0.0", false},
		{"v1.1", "v1.0.0", true},

		// no leading v
		{"0.9.0", "v0.8.0", true},

		// garbage → never newer
		{"", "v0.8.0", false},
		{"v0.8.0", "", false},
		{"abc", "v0.8.0", false},
		{"v0.8.x", "v0.8.0", false},
		{"v0..0", "v0.8.0", false},
		{"v-1.0.0", "v0.8.0", false},
	}
	for _, c := range cases {
		got := isNewer(c.latest, c.current)
		if got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.8.0", "v0.8.0"},
		{"0.8.0", "v0.8.0"},
		{"v0.8.0\n", "v0.8.0"},
		{"  v0.8.0  ", "v0.8.0"},
		{"v0.8.0-rc1", "v0.8.0"}, // suffix stripped
		{"v1", "v1.0.0"},         // missing segments filled
		{"v1.2", "v1.2.0"},
		{"", ""},
		{"abc", ""},
		{"v9.9.9.9", ""}, // too many segments
	}
	for _, c := range cases {
		got := normalizeTag(c.in)
		if got != c.want {
			t.Errorf("normalizeTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
