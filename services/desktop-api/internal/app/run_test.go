package app

import "testing"

func TestAllowedOrigin(t *testing.T) {
	for _, tc := range []struct {
		origin  string
		channel string
		want    bool
	}{
		{"one-desktop://app", "release", true},
		{"one-desktop://app", "development", true},
		{"http://127.0.0.1:5173", "development", true},
		{"http://127.0.0.1:5173", "release", false},
		{"null", "release", false},
		{"file://", "release", false},
		{"one-desktop://app.evil", "release", false},
		{"one-desktop://app:80", "release", false},
		{"https://example.com", "development", false},
		{"tauri://localhost", "release", false},
	} {
		t.Run(tc.origin+"/"+tc.channel, func(t *testing.T) {
			if got := allowedOrigin(tc.origin, tc.channel); got != tc.want {
				t.Fatalf("allowedOrigin(%q, %q) = %v, want %v", tc.origin, tc.channel, got, tc.want)
			}
		})
	}
}
