// Package linear implements the built-in Linear integration.
package linear

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
)

type Plugin struct {
	client       *http.Client
	apiURL       string
	tokenURL     string
	authorizeURL string
	root         string
	mu           sync.Mutex
	auth         *authAttempt
	cache        map[string]tracking.Issues
}

func New() *Plugin {
	return &Plugin{client: &http.Client{Timeout: 15 * time.Second}, apiURL: "https://api.linear.app/graphql", tokenURL: "https://api.linear.app/oauth/token", authorizeURL: "https://linear.app/oauth/authorize", cache: map[string]tracking.Issues{}}
}
func (*Plugin) ID() string { return "linear" }
func (p *Plugin) Contributions() plugins.Contributions {
	return plugins.Contributions{IssueTracking: p}
}

func (p *Plugin) Status(ctx context.Context) (tracking.ConnectionState, error) {
	result := tracking.ConnectionState{Connections: []tracking.Connection{}}
	err := p.transaction(ctx, func(doc *document) (bool, error) {
		result.Config = doc.Config
		for _, connection := range doc.Connections {
			result.Connections = append(result.Connections, connection.Connection)
		}
		return false, nil
	})
	if err != nil {
		return result, err
	}
	sort.Slice(result.Connections, func(i, j int) bool { return result.Connections[i].ID < result.Connections[j].ID })
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.auth != nil {
		copy := p.auth.public
		result.Authorization = &copy
	}
	return result, nil
}

func (p *Plugin) Configure(ctx context.Context, cfg tracking.OAuthConfig) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	p.CancelAuthorization()
	return p.transaction(ctx, func(doc *document) (bool, error) { doc.Config = cfg; return true, nil })
}

func (p *Plugin) Disconnect(ctx context.Context, id string) error {
	err := p.transaction(ctx, func(doc *document) (bool, error) { delete(doc.Connections, id); return true, nil })
	if err == nil {
		p.clearCache()
	}
	return err
}

func (p *Plugin) clearCache() { p.mu.Lock(); p.cache = map[string]tracking.Issues{}; p.mu.Unlock() }

var _ tracking.Backend = (*Plugin)(nil)
