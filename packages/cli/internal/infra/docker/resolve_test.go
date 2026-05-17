package docker

// resolve_test.go locks the host / namespace derivation rules that
// are the only real per-kind difference between docker / dockerhub /
// ghcr / acr. Drift here corrupts the image tag silently — build
// succeeds, push lands at the wrong host — so each path stays pinned.

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
)

func TestHostForKind(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		cp         *profile.ContainerProfile
		wantHost   string
		wantErr    bool
		wantErrSub string
	}{
		{
			name:     "docker with bare host",
			kind:     "docker",
			cp:       &profile.ContainerProfile{Registry: "harbor.example.com"},
			wantHost: "harbor.example.com",
		},
		{
			name:     "docker strips https scheme",
			kind:     "docker",
			cp:       &profile.ContainerProfile{Registry: "https://harbor.example.com"},
			wantHost: "harbor.example.com",
		},
		{
			name:     "docker strips http scheme",
			kind:     "docker",
			cp:       &profile.ContainerProfile{Registry: "http://harbor.example.com"},
			wantHost: "harbor.example.com",
		},
		{
			name:       "docker requires registry",
			kind:       "docker",
			cp:         &profile.ContainerProfile{},
			wantErr:    true,
			wantErrSub: "registry",
		},
		{
			name:     "dockerhub host is fixed",
			kind:     "dockerhub",
			cp:       &profile.ContainerProfile{Registry: "ignored.example.com"},
			wantHost: "index.docker.io",
		},
		{
			name:     "ghcr host is fixed",
			kind:     "ghcr",
			cp:       &profile.ContainerProfile{},
			wantHost: "ghcr.io",
		},
		{
			name:     "acr derives from region",
			kind:     "acr",
			cp:       &profile.ContainerProfile{Region: "cn-hangzhou"},
			wantHost: "registry.cn-hangzhou.aliyuncs.com",
		},
		{
			name:     "acr region trims whitespace",
			kind:     "acr",
			cp:       &profile.ContainerProfile{Region: "  cn-shanghai  "},
			wantHost: "registry.cn-shanghai.aliyuncs.com",
		},
		{
			name:       "acr requires region",
			kind:       "acr",
			cp:         &profile.ContainerProfile{},
			wantErr:    true,
			wantErrSub: "region",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := hostForKind(tt.kind, tt.cp)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("hostForKind(%q): expected error, got host=%q", tt.kind, host)
				}
				if tt.wantErrSub != "" && !contains(err.Error(), tt.wantErrSub) {
					t.Errorf("hostForKind(%q): error %q does not mention %q", tt.kind, err.Error(), tt.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("hostForKind(%q): unexpected error: %v", tt.kind, err)
			}
			if host != tt.wantHost {
				t.Errorf("hostForKind(%q): got %q, want %q", tt.kind, host, tt.wantHost)
			}
		})
	}
}

func TestDefaultNamespaceForKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		registry string
		username string
		want     string
	}{
		{"dockerhub defaults to username", "dockerhub", "", "alice", "alice"},
		{"dockerhub empty username", "dockerhub", "", "", ""},
		{"ghcr defaults to username", "ghcr", "", "bob", "bob"},
		{"ghcr empty username", "ghcr", "", "", ""},
		{"acr never defaults", "acr", "", "carol", ""},
		{"docker (generic) known host docker.io", "docker", "docker.io", "dave", "dave"},
		{"docker (generic) known host ghcr.io", "docker", "ghcr.io", "dave", "dave"},
		{"docker (generic) private host", "docker", "harbor.example.com", "dave", ""},
		{"docker (generic) scheme-prefixed", "docker", "https://ghcr.io", "dave", "dave"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultNamespaceForKind(tt.kind, tt.registry, tt.username)
			if got != tt.want {
				t.Errorf("defaultNamespaceForKind(%q, %q, %q): got %q, want %q",
					tt.kind, tt.registry, tt.username, got, tt.want)
			}
		})
	}
}

func TestResolveRegistry_UnknownKind(t *testing.T) {
	_, err := ResolveRegistry(ResolveRegistryInput{
		ProjectRoot: t.TempDir(),
		Kind:        "totally-not-a-kind",
	})
	if err == nil {
		t.Fatalf("expected CONTAINER_KIND_UNKNOWN error, got nil")
	}
	if !contains(err.Error(), "totally-not-a-kind") {
		t.Errorf("error should mention the unknown kind, got: %v", err)
	}
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
