package infisical

import (
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "/"},
		{".", "/"},
		{"/", "/"},
		{"x", "/x"},
		{"x/y", "/x/y"},
		{"/x/y", "/x/y"},
		{"/x/y/", "/x/y"},
		{"//x///y/", "/x/y"},
		{`x\y`, "/x/y"},
	}
	for _, tc := range cases {
		if got := NormalizePath(tc.in); got != tc.want {
			t.Errorf("NormalizePath(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitizeEnvName(t *testing.T) {
	good := []string{"dev", "staging", "prod", "review-123", "qa_2"}
	for _, s := range good {
		if _, err := SanitizeEnvName(s); err != nil {
			t.Errorf("SanitizeEnvName(%q) unexpected err: %v", s, err)
		}
	}
	bad := []string{"", " ", "_x", "-x", "x y", "x/y", "x\tcolon"}
	for _, s := range bad {
		if _, err := SanitizeEnvName(s); err == nil {
			t.Errorf("SanitizeEnvName(%q) expected error, got none", s)
		}
	}
}

func TestAssertValidKey(t *testing.T) {
	good := []string{"DATABASE_URL", "_PRIVATE", "k", "X1", "MY_KEY_2"}
	for _, k := range good {
		if err := AssertValidKey(k); err != nil {
			t.Errorf("AssertValidKey(%q) unexpected err: %v", k, err)
		}
	}
	bad := []string{"", "1KEY", "MY-KEY", "MY KEY", "key.path"}
	for _, k := range bad {
		if err := AssertValidKey(k); err == nil {
			t.Errorf("AssertValidKey(%q) expected error, got none", k)
		}
	}
}

func TestPathInheritanceChain(t *testing.T) {
	cases := []struct {
		name     string
		root     string
		path     string
		inherits bool
		want     []string
	}{
		{
			name:     "no inherits returns leaf only",
			root:     "/",
			path:     "/services/user-api",
			inherits: false,
			want:     []string{"/services/user-api"},
		},
		{
			name:     "inherits walks segments from root",
			root:     "/",
			path:     "/services/user-api",
			inherits: true,
			want:     []string{"/", "/services", "/services/user-api"},
		},
		{
			name:     "inherits with non-default rootPath",
			root:     "/team-foo",
			path:     "/team-foo/web/dashboard",
			inherits: true,
			want:     []string{"/team-foo", "/team-foo/web", "/team-foo/web/dashboard"},
		},
		{
			name:     "path equals root",
			root:     "/",
			path:     "/",
			inherits: true,
			want:     []string{"/"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pathInheritanceChain(tc.root, tc.path, tc.inherits)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestResolveSubprojectPath_DefaultsFromRelativeDir(t *testing.T) {
	cfg := &WorkspaceConfig{RootPath: "/"}
	sub := &workspace.Project{Name: "user-api", RelativeDir: "services/user-api"}
	res := ResolveSubprojectPath(cfg, sub, nil)
	if res.Path != "/services/user-api" {
		t.Errorf("default path wrong: %v", res.Path)
	}
	if !res.Inherits {
		t.Errorf("default Inherits should be true")
	}
}

func TestResolveSubprojectPath_OverrideWins(t *testing.T) {
	cfg := &WorkspaceConfig{RootPath: "/"}
	sub := &workspace.Project{Name: "user-api", RelativeDir: "services/user-api"}
	off := false
	res := ResolveSubprojectPath(cfg, sub, &SubprojectConfig{
		Path:     "/teams/payments",
		Inherits: &off,
	})
	if res.Path != "/teams/payments" {
		t.Errorf("override path ignored: %v", res.Path)
	}
	if res.Inherits {
		t.Errorf("Inherits override ignored")
	}
	if !reflect.DeepEqual(res.Chain, []string{"/teams/payments"}) {
		t.Errorf("chain wrong with inherits=false: %v", res.Chain)
	}
}
