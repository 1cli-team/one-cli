package miseconfig

import (
	"strings"
	"testing"
)

func TestSourceDependencyClosureIncludesLibrariesWithoutBuild(t *testing.T) {
	edges := map[string][]string{"web": {"middle"}, "middle": {"base"}}
	deps, err := dependencyClosure("web", edges)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deps, ",") != "base,middle" {
		t.Fatal(deps)
	}
	edges["base"] = []string{"web"}
	if _, err = dependencyClosure("web", edges); err == nil {
		t.Fatal("source dependency cycle accepted")
	}
}
