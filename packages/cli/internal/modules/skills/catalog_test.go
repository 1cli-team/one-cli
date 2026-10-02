package skills

import (
	"reflect"
	"testing"
)

func TestTemplateAdaptationSkillsStayScoped(t *testing.T) {
	for _, tt := range []struct {
		template string
		repo     string
		want     []string
	}{
		{"nestjs-api", "local:nestjs-api", []string{"one-cli", "one-nestjs"}},
		{"go-api", "local:go-api", []string{"one-cli", "one-go"}},
		{"go-lib", "local:go-lib", []string{"one-cli", "one-go"}},
		{"expo-mobile", "local:expo-mobile", []string{"one-cli", "one-expo"}},
		{"react-spa", "local:react-spa", []string{"one-cli", "one-web"}},
		{"nextjs-app", "local:nextjs-app", []string{"one-cli", "one-web"}},
		{"nextjs-site", "local:nextjs-site", []string{"one-cli", "one-web"}},
		{"fumadocs-docs", "local:fumadocs-docs", []string{"one-cli", "one-web", "one-fumadocs"}},
		{"electron-app", "local:electron-app", []string{"one-cli", "one-electron", "one-web"}},
		{"ts-library", "local:ts-library", []string{"one-cli"}},
		{"empty-app", "local:empty-app", []string{"one-cli"}},
		{"empty-service", "local:empty-service", []string{"one-cli"}},
		{"empty-library", "local:empty-library", []string{"one-cli"}},
		{"nextjs-app", "https://example.com/custom-next", []string{"one-cli"}},
		{"custom", "local:custom", []string{"one-cli"}},
		{"", "", []string{"one-cli"}},
	} {
		t.Run(tt.template+"/"+tt.repo, func(t *testing.T) {
			var names []string
			for _, selection := range Defaults(tt.template, tt.repo) {
				if selection.Source == "" {
					names = append(names, selection.Names...)
				}
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("template adaptation skills = %v, want %v", names, tt.want)
			}
		})
	}
}
