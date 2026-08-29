package container

import "testing"

func TestRegistryHasCredentials(t *testing.T) {
	tests := []struct {
		name string
		r    *Registry
		want bool
	}{
		{"nil", nil, false},
		{"empty", &Registry{}, false},
		{"only host", &Registry{Registry: "ghcr.io"}, false},
		{"only user", &Registry{Registry: "ghcr.io", Username: "u"}, false},
		{"full", &Registry{Registry: "ghcr.io", Username: "u", Password: "p"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasCredentials(); got != tt.want {
				t.Fatalf("HasCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageTagVersion(t *testing.T) {
	tests := map[string]string{
		"api:v1.2.3":                     "v1.2.3",
		"ghcr.io/team/api:latest":        "latest",
		"localhost:5000/team/api:v2.0.0": "v2.0.0",
		"ghcr.io/team/api":               "",
	}
	for reference, want := range tests {
		if got := ImageTagVersion(reference); got != want {
			t.Errorf("ImageTagVersion(%q) = %q, want %q", reference, got, want)
		}
	}
}
