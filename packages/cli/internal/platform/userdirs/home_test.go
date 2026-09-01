package userdirs

import (
	"path/filepath"
	"testing"
)

func TestHomeHonorsExplicitHomeOnEveryPlatform(t *testing.T) {
	want := filepath.Join(t.TempDir(), "isolated-home")
	t.Setenv("HOME", want)
	got, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if got != want {
		t.Fatalf("Home = %q, want %q", got, want)
	}
}
