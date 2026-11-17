// Package gateway defines the Desktop-facing API of the One Gateway.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

type Status string

const (
	Processing Status = "processing"
	Submitted  Status = "submitted"
	Failed     Status = "failed"
)

type Jobs struct {
	Enabled bool              `json:"enabled"`
	Issues  map[string]Status `json:"issues"`
}

const SnapshotSchema = "one-gateway/status/v1"

type QueueCounts struct {
	Queued    int `json:"queued"`
	Active    int `json:"active"`
	Review    int `json:"review"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type ProjectSummary struct {
	ProjectID    string                  `json:"projectId"`
	GitHub       string                  `json:"github"`
	Repository   gitmodule.RepositoryRef `json:"repository,omitzero"`
	Base         string                  `json:"base"`
	Repositories []RepositorySummary     `json:"repositories,omitempty"`
}

type RepositorySummary struct {
	GitHub     string                  `json:"github"`
	Repository gitmodule.RepositoryRef `json:"repository,omitzero"`
	Base       string                  `json:"base"`
}

type Snapshot struct {
	Schema    string           `json:"schema"`
	StartedAt time.Time        `json:"startedAt"`
	Projects  int              `json:"projects"`
	Mappings  []ProjectSummary `json:"mappings"`
	Jobs      QueueCounts      `json:"jobs"`
}

var ErrInvalidSnapshot = errors.New("invalid Gateway status response")

type ResponseError struct {
	Code   int
	Reason string
}

func (e *ResponseError) Error() string { return fmt.Sprintf("Gateway HTTP %d", e.Code) }

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

func NewClient(endpoint, token string) (*Client, error) {
	base, err := url.Parse(endpoint)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("invalid Gateway URL")
	}
	loopback := net.ParseIP(base.Hostname()).IsLoopback() || base.Hostname() == "localhost"
	if base.Scheme != "https" && (base.Scheme != "http" || !loopback) {
		return nil, fmt.Errorf("Gateway requires HTTPS or loopback HTTP")
	}
	if !ValidToken(token) {
		return nil, fmt.Errorf("invalid Gateway token")
	}
	return &Client{
		base: base, token: token,
		http: &http.Client{
			Timeout:       45 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func ValidToken(token string) bool {
	return len(token) >= 32 && !strings.ContainsAny(token, " \t\r\n")
}

func (c *Client) List(ctx context.Context, project string) (Jobs, error) {
	var jobs Jobs
	err := c.do(ctx, http.MethodGet, &jobs, "projects", url.PathEscape(project), "jobs")
	return jobs, err
}

func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	if err := c.do(ctx, http.MethodGet, &result, "status"); err != nil {
		return Snapshot{}, err
	}
	if result.Schema != SnapshotSchema || result.StartedAt.IsZero() {
		return Snapshot{}, ErrInvalidSnapshot
	}
	return result, nil
}

func (c *Client) do(ctx context.Context, method string, result any, path ...string) error {
	return c.request(ctx, method, nil, result, path...)
}

func (c *Client) request(ctx context.Context, method string, payload, result any, path ...string) error {
	parts := append([]string{"v1"}, path...)
	return c.send(ctx, method, c.base.JoinPath(parts...), payload, result)
}

func (c *Client) send(ctx context.Context, method string, endpoint *url.URL, payload, result any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var problem struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&problem)
		return &ResponseError{Code: resp.StatusCode, Reason: problem.Reason}
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(result)
}
