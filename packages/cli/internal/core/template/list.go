package template

import (
	"context"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ListResult is the JSON payload for `one templates`. Schema:
//
//	{ "schema": "one-cli/templates/v1", "total": N, "templates": [...] }
type ListResult struct {
	Schema    string     `json:"schema"`
	Total     int        `json:"total"`
	Templates []Template `json:"templates"`
}

// RenderTTY prints a human-friendly table of the available templates,
// grouped by category and sorted by id within each group. Hidden when
// stdout is non-TTY (output.Emit auto-dispatches based on mode).
func (r *ListResult) RenderTTY(w io.Writer) {
	if r == nil || len(r.Templates) == 0 {
		fmt.Fprintln(w, i18n.T("templates.empty"))
		return
	}
	// Group by category, then sort by id within each category. The
	// outer category order matches the canonical sequence used by
	// `one add`'s interactive picker.
	order := []string{"frontend", "backend", "library"}
	seen := map[string]bool{}
	for _, c := range order {
		seen[c] = true
	}
	grouped := map[string][]Template{}
	for _, t := range r.Templates {
		c := string(t.Category)
		grouped[c] = append(grouped[c], t)
		if !seen[c] {
			order = append(order, c)
			seen[c] = true
		}
	}
	for _, group := range grouped {
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("templates.headings"))
	for _, cat := range order {
		for _, t := range grouped[cat] {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", displayCategory(cat), t.ID, t.Toolchain, t.DisplayDescription())
		}
	}
	tw.Flush()
	fmt.Fprintf(w, i18n.T("templates.count"), r.Total)
}

// List returns the registry payload ready to emit. It does not consult
// any flags — the caller decides JSON vs TTY rendering.
func List(ctx context.Context) (*ListResult, error) {
	registry, err := Fetch(ctx, "")
	if err != nil {
		return nil, err
	}
	return &ListResult{
		Schema:    "one-cli/templates/v1",
		Total:     len(registry.Templates),
		Templates: registry.Templates,
	}, nil
}

func (t Template) DisplayName() string {
	return i18n.LocalizedValue("template."+t.ID+".name", t.Name)
}

func (t Template) DisplayDescription() string {
	return i18n.LocalizedValue("template."+t.ID+".description", t.Description)
}

func displayCategory(category string) string {
	key := "template.category." + category
	if label := i18n.T(key); label != key {
		return label
	}
	return category
}
