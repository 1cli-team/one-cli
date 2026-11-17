package gateway

import (
	"context"
	"net/http"
)

type CodexModelCatalog struct {
	Models        []CodexModel `json:"models"`
	DefaultModel  string       `json:"defaultModel,omitempty"`
	DefaultEffort string       `json:"defaultEffort,omitempty"`
}

type CodexModel struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Efforts []string `json:"efforts"`
}

func (c *Client) CodexModels(ctx context.Context) (CodexModelCatalog, error) {
	var catalog CodexModelCatalog
	err := c.do(ctx, http.MethodGet, &catalog, "codex", "models")
	return catalog, err
}
