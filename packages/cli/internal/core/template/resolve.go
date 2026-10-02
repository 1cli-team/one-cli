package template

import (
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Resolve keeps retired built-in IDs actionable without silently selecting a
// different framework. An explicit registry entry always takes precedence.
func (r *Registry) Resolve(id string) (*Template, error) {
	ids := make([]string, 0, len(r.Templates))
	for i := range r.Templates {
		entry := &r.Templates[i]
		if entry.ID == id {
			return entry, nil
		}
		ids = append(ids, entry.ID)
	}
	replacement := map[string]string{"astro-site": "nextjs-site", "starlight-docs": "fumadocs-docs"}[id]
	context := map[string]any{"requested_template": id, "available_templates": ids}
	message := i18n.Tf("add.template_missing", id)
	for _, entry := range r.Templates {
		if replacement != "" && entry.ID == replacement {
			context["replacement_template"] = replacement
			message = i18n.Tf("template.retired", id, replacement)
			break
		}
	}
	return nil, cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND, message).WithContext(context)
}
