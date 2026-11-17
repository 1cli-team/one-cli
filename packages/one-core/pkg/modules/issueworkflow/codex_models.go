package issueworkflow

import (
	"context"
	"fmt"
	"os"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func (c Codex) CodexModels(ctx context.Context) (api.CodexModelCatalog, error) {
	tmp, err := os.MkdirTemp(c.StateDir, "codex-models-")
	if err != nil {
		return api.CodexModelCatalog{}, err
	}
	defer os.RemoveAll(tmp)
	session, err := c.start(ctx, tmp, "", tmp)
	if err != nil {
		return api.CodexModelCatalog{}, err
	}
	defer session.stop()
	return session.modelCatalog()
}

func (s *codexSession) modelCatalog() (api.CodexModelCatalog, error) {
	var settings struct {
		Config struct {
			Model    string `json:"model"`
			Provider string `json:"model_provider"`
			Effort   string `json:"model_reasoning_effort"`
		} `json:"config"`
	}
	if err := s.call("config/read", map[string]bool{"includeLayers": false}, &settings, nil); err != nil {
		return api.CodexModelCatalog{}, err
	}
	catalog := api.CodexModelCatalog{
		Models: []api.CodexModel{}, DefaultModel: settings.Config.Model, DefaultEffort: settings.Config.Effort,
	}
	params := map[string]any{"limit": 100, "includeHidden": false}
	if settings.Config.Provider != "" {
		params["modelProvider"] = settings.Config.Provider
	}
	seen := map[string]bool{}
	for {
		var page struct {
			Data []struct {
				ID        string `json:"id"`
				Model     string `json:"model"`
				Name      string `json:"displayName"`
				IsDefault bool   `json:"isDefault"`
				Efforts   []struct {
					Value string `json:"reasoningEffort"`
				} `json:"supportedReasoningEfforts"`
			} `json:"data"`
			NextCursor string `json:"nextCursor"`
		}
		if err := s.call("model/list", params, &page, nil); err != nil {
			return api.CodexModelCatalog{}, err
		}
		for _, item := range page.Data {
			id := item.Model
			if id == "" {
				id = item.ID
			}
			model := api.CodexModel{ID: id, Name: item.Name, Efforts: []string{}}
			for _, effort := range item.Efforts {
				model.Efforts = append(model.Efforts, effort.Value)
			}
			if catalog.DefaultModel == "" && item.IsDefault {
				catalog.DefaultModel = id
			}
			catalog.Models = append(catalog.Models, model)
		}
		if page.NextCursor == "" {
			return catalog, nil
		}
		if seen[page.NextCursor] {
			return api.CodexModelCatalog{}, fmt.Errorf("Codex model list repeated a pagination cursor")
		}
		seen[page.NextCursor] = true
		params["cursor"] = page.NextCursor
	}
}
