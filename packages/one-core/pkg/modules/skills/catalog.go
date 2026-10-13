// Package skills installs curated development skills and runs the upstream
// Skills CLI. Business skills are managed exclusively by explicit user commands.
package skills

import "strings"

const CLIVersion = "1.7.0"

const oneSource = "1cli-team/one-cli"

// Release builds inject their full Git commit through ldflags. Development and
// snapshot builds leave this empty so unpublished changes use local copies.
var bundledSourceRef = ""

// Selection uses the skill's frontmatter name, not its source directory name.
// An empty Source selects One's usage/template skills for this CLI build.
type Selection struct {
	Source string
	Names  []string
}

const (
	findSource  = "vercel-labs/skills#3694740352eeef5cdd689af694c485f1ff62eec3"
	reactSource = "vercel-labs/agent-skills#063bee94c3f4df8453406c830b0a7df0f2860278"
	antfuSource = "antfu/skills#e53a142a2420e8cd812cfe9ed0484ab01bc856aa"
	uiSource    = "shadcn-ui/ui#295a1f114a138f23b5dfee0e0c6812394dfeb90c"
	tsSource    = "rolldown/tsdown#eb40c95efdf7f98d0aa7b0178a3b7322986bd419"
	expoSource  = "expo/skills#13ad8e05874195633b5c185f6947bb6400e228fc"
	goSource    = "affaan-m/ecc#ef648e01899ba3e8dc6371642deaaf64b4477775"
	nestSource  = "kadajett/agent-nestjs-skills#3986e0cede33958e000f959031cbee0cd83c2941"
)

// Defaults returns only development skills for known built-in templates.
// Empty/custom templates get common workspace guidance without guessed stacks.
func Defaults(templateID, repo string) []Selection {
	plan := []Selection{{Names: []string{"one-cli"}}, {Source: findSource, Names: []string{"find-skills"}}}
	if repo != "local:"+templateID {
		return plan
	}
	node := Selection{Source: antfuSource, Names: []string{"pnpm"}}
	web := []Selection{
		{Source: reactSource, Names: []string{"vercel-react-best-practices", "vercel-composition-patterns"}},
		{Source: uiSource, Names: []string{"shadcn"}},
	}
	switch templateID {
	case "go-api", "go-lib":
		plan[0].Names = append(plan[0].Names, "one-go")
		plan = append(plan, Selection{Source: goSource, Names: []string{"golang-patterns", "golang-testing"}})
	case "nestjs-api":
		plan[0].Names = append(plan[0].Names, "one-nestjs")
		plan = append(plan, node, Selection{Source: nestSource, Names: []string{"nestjs-best-practices"}})
	case "nextjs-app", "nextjs-site", "fumadocs-docs":
		plan[0].Names = append(plan[0].Names, "one-web")
		plan = append(plan, node)
		plan = append(plan, web...)
		if templateID == "fumadocs-docs" {
			plan[0].Names = append(plan[0].Names, "one-fumadocs")
		}
	case "react-spa":
		plan[0].Names = append(plan[0].Names, "one-web")
		node.Names = append(node.Names, "vite")
		plan = append(plan, node)
		plan = append(plan, web...)
	case "expo-mobile":
		plan[0].Names = append(plan[0].Names, "one-expo")
		plan = append(plan, node, Selection{Source: expoSource, Names: []string{
			"expo-overview", "expo-router", "expo-data-fetching", "expo-dev-client", "expo-upgrade", "expo-project-structure",
		}})
	case "ts-library":
		plan = append(plan, node, Selection{Source: tsSource, Names: []string{"tsdown"}})
	case "electron-app":
		plan[0].Names = append(plan[0].Names, "one-electron", "one-web")
		node.Names = append(node.Names, "vite")
		plan = append(plan, node, Selection{Source: tsSource, Names: []string{"tsdown"}})
		plan = append(plan, web...)
	}
	return plan
}

func sourceRepo(source string) string {
	repo, _, _ := strings.Cut(source, "#")
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) == 3 {
		return parts[0] + "/" + parts[1]
	}
	return repo
}

func bundledSource() string {
	if bundledSourceRef == "" {
		return "./.agents/skills"
	}
	return oneSource + "/packages/agent-skills#" + bundledSourceRef
}
