package template

import (
	"testing"
)

func TestCheckAllowedBackends_NoConstraint(t *testing.T) {
	tpl := Template{ID: "foo"}
	got := CheckAllowedBackends(tpl, map[string]string{"deploy": "deploy/k8s"}, "")
	if len(got) != 0 {
		t.Errorf("template without compat should never warn, got %v", got)
	}
}

func TestCheckAllowedBackends_EmptySelection(t *testing.T) {
	tpl := Template{
		ID:     "foo",
		Compat: map[string][]string{"deploy": {"vercel"}},
	}
	got := CheckAllowedBackends(tpl, nil, "")
	if len(got) != 0 {
		t.Errorf("empty selection means no workspace policy → no warnings, got %v", got)
	}
}

func TestCheckAllowedBackends_OptedOut(t *testing.T) {
	tpl := Template{
		ID:     "starlight-docs",
		Compat: map[string][]string{"deploy": {"vercel", "s3"}},
	}
	// User explicitly skipped deploy at create time → "deploy" is not
	// a key in selection. No warning.
	got := CheckAllowedBackends(tpl, map[string]string{"env": "env/dotenv"}, "")
	if len(got) != 0 {
		t.Errorf("opted-out domain should not warn, got %v", got)
	}
}

func TestCheckAllowedBackends_EmptyAllowedList(t *testing.T) {
	tpl := Template{
		ID:     "expo-mobile",
		Compat: map[string][]string{"deploy": {}},
	}
	// `[]` means "template doesn't participate" — no warning even
	// if workspace has a deploy plugin selected.
	got := CheckAllowedBackends(tpl, map[string]string{"deploy": "deploy/k8s"}, "")
	if len(got) != 0 {
		t.Errorf("empty allowed list should not warn, got %v", got)
	}
}

func TestCheckAllowedBackends_Compatible(t *testing.T) {
	tpl := Template{
		ID:     "go-api",
		Compat: map[string][]string{"deploy": {"k8s", "docker-compose"}},
	}
	got := CheckAllowedBackends(tpl, map[string]string{"deploy": "deploy/k8s"}, "")
	if len(got) != 0 {
		t.Errorf("compatible selection should not warn, got %v", got)
	}
}

func TestCheckAllowedBackends_Incompatible(t *testing.T) {
	tpl := Template{
		ID:     "starlight-docs",
		Compat: map[string][]string{"deploy": {"vercel", "s3"}},
	}
	got := CheckAllowedBackends(tpl, map[string]string{"deploy": "deploy/k8s"}, "")
	if len(got) != 1 {
		t.Fatalf("expected 1 warning, got %d (%v)", len(got), got)
	}
	w := got[0]
	if w.Domain != "deploy" || w.SelectedID != "deploy/k8s" || w.TemplateID != "starlight-docs" {
		t.Errorf("warning fields wrong: %+v", w)
	}
	if w.SubprojectName != "" {
		t.Errorf("expected empty SubprojectName for add-time check, got %q", w.SubprojectName)
	}
	msg := w.Message()
	if msg == "" {
		t.Error("Message() should not be empty")
	}
}

func TestCheckAllowedBackends_WithSubprojectName(t *testing.T) {
	tpl := Template{
		ID:     "starlight-docs",
		Compat: map[string][]string{"deploy": {"vercel"}},
	}
	got := CheckAllowedBackends(tpl, map[string]string{"deploy": "deploy/k8s"}, "my-docs")
	if len(got) != 1 || got[0].SubprojectName != "my-docs" {
		t.Errorf("expected SubprojectName=my-docs, got %+v", got)
	}
}

func TestCheckAllowedBackends_DeterministicOrder(t *testing.T) {
	tpl := Template{
		ID: "weird",
		Compat: map[string][]string{
			"deploy": {"deploy/k8s"},
			"ci":     {"ci/github-actions"},
		},
	}
	selection := map[string]string{
		"deploy": "deploy/vercel", // mismatch → warning
		"ci":     "ci/circleci",   // mismatch (hypothetical) → warning
	}
	got := CheckAllowedBackends(tpl, selection, "")
	if len(got) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(got))
	}
	// Domains should be alphabetical: ci, deploy.
	if got[0].Domain != "ci" || got[1].Domain != "deploy" {
		t.Errorf("warnings not in alphabetical domain order: %v %v",
			got[0].Domain, got[1].Domain)
	}
}
