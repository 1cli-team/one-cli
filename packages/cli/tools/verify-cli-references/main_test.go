package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTaskShorthandReferencesAcrossDocuments(t *testing.T) {
	root := &cobra.Command{Use: "one"}
	root.AddCommand(&cobra.Command{Use: "run"}, &cobra.Command{Use: "env"})
	commands := collectCommands(root)
	documents := map[string][]byte{
		"tasks.md":       []byte("`one run dev`, `one run build`, `one run lint`, `one run docs:build`"),
		"quick-start.md": []byte("```sh\none dev -- --port 4300\none build\none lint\none docs:build\none env\n```"),
	}
	valid := collectReferences(commands, documents)
	for path, content := range documents {
		if problems := scanFile(path, content, valid); len(problems) != 0 {
			t.Fatal(problems)
		}
	}
	for _, name := range []string{"dev", "build", "lint", "docs:build"} {
		if _, ok := commands[name]; ok {
			t.Fatalf("task %q was added to the built-in command catalogue", name)
		}
	}
	problems := scanFile("typo.md", []byte("`one buidl`"), valid)
	if len(problems) != 1 || !strings.Contains(problems[0], "buidl") {
		t.Fatalf("expected a reference error for undeclared task typo: %v", problems)
	}
}
