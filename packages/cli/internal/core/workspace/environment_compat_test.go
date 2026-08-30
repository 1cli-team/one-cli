package workspace

import (
	"reflect"
	"testing"
)

func TestProfileBindingEnvironmentMapsLegacyPreviewToStaging(t *testing.T) {
	legacy := &Manifest{Environments: &Environments{
		Names: []string{"dev", "staging", "prod"}, Default: "dev",
	}}
	if got := ProfileBindingEnvironment(legacy, "preview"); got != "staging" {
		t.Fatalf("legacy preview binding key = %q; want staging", got)
	}
	if got := ProfileBindingEnvironment(legacy, "prod"); got != "prod" {
		t.Fatalf("prod binding key = %q", got)
	}
	modern := &Manifest{Environments: &Environments{
		Names: []string{"dev", "preview", "prod"}, Default: "dev",
	}}
	if got := ProfileBindingEnvironment(modern, "preview"); got != "preview" {
		t.Fatalf("modern preview binding key = %q", got)
	}
	mixed := &Manifest{Environments: &Environments{
		Names: []string{"dev", "preview", "staging", "prod"}, Default: "dev",
	}}
	if got := ProfileBindingEnvironment(mixed, "preview"); got != "preview" {
		t.Fatalf("explicit preview must win over staging; got %q", got)
	}
}

func TestNewWorkspaceDefaultEnvironmentsUsePreview(t *testing.T) {
	want := []string{"dev", "preview", "prod"}
	if !reflect.DeepEqual(DefaultEnvironments, want) {
		t.Fatalf("DefaultEnvironments = %#v; want %#v", DefaultEnvironments, want)
	}
}
