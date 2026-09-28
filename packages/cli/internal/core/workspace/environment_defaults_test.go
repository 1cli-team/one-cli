package workspace

import (
	"reflect"
	"testing"
)

func TestNewWorkspaceDefaultEnvironmentsUsePreview(t *testing.T) {
	want := []string{"dev", "preview", "prod"}
	if !reflect.DeepEqual(DefaultEnvironments, want) {
		t.Fatalf("DefaultEnvironments = %#v; want %#v", DefaultEnvironments, want)
	}
}
