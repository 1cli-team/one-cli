package miseconfig

import (
	"sort"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func dependencyClosure(name string, edges map[string][]string) ([]string, error) {
	seen := map[string]int{}
	result := []string{}
	var walk func(string) error
	walk = func(current string) error {
		if seen[current] == 1 {
			return i18n.Errorf("build.dependency_cycle", name+" -> "+current)
		}
		if seen[current] == 2 {
			return nil
		}
		seen[current] = 1
		for _, dep := range edges[current] {
			if err := walk(dep); err != nil {
				return err
			}
		}
		seen[current] = 2
		if current != name {
			result = append(result, current)
		}
		return nil
	}
	err := walk(name)
	sort.Strings(result)
	return result, err
}
