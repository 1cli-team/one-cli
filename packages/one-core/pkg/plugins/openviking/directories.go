package openviking

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

func resolveDirectory(ctx context.Context, saved connection, uri string, createMissing bool) (resourceURI, error) {
	dir, valid := parseResourceURI(uri)
	if !valid {
		return resourceURI{}, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_uri"))
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return resourceURI{}, err
	}
	info, err := stat(ctx, client, dir.value)
	var apiErr *ov.Error
	if createMissing && errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound && apiErr.Code == "NOT_FOUND" {
		if err := client.Mkdir(ctx, dir.value, ""); err != nil {
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict {
				return resourceURI{}, sdkError(err)
			}
		}
		info, err = stat(ctx, client, dir.value)
	}
	if err != nil {
		return resourceURI{}, sdkError(err)
	}
	if !info.IsDir {
		return resourceURI{}, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.directory_required"))
	}
	canonical, ok := parseResourceURI(info.URI)
	if !ok {
		return resourceURI{}, invalidResponse()
	}
	return canonical, nil
}

func (p *Plugin) Check(ctx context.Context, workspaceID string) (knowledge.Binding, error) {
	saved, root, err := p.bindingFor(ctx, workspaceID)
	if err != nil {
		return knowledge.Binding{}, err
	}
	directory, err := resolveDirectory(ctx, saved, root.value, false)
	if err != nil {
		return knowledge.Binding{}, err
	}
	return knowledge.Binding{URI: directory.value}, nil
}

func (*Plugin) Browse(ctx context.Context, uri string) (knowledge.Directories, error) {
	dirName, err := configDirectory()
	if err != nil {
		return knowledge.Directories{}, err
	}
	saved, err := loadConnection(ctx, dirName)
	if err != nil {
		return knowledge.Directories{}, err
	}
	if err := requireConnection(saved); err != nil {
		return knowledge.Directories{}, err
	}
	if strings.TrimSpace(uri) == "" {
		uri = "viking://resources"
	}
	dir, err := resolveDirectory(ctx, saved, uri, false)
	if err != nil {
		return knowledge.Directories{}, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Directories{}, err
	}
	found, err := client.List(ctx, dir.value, &ov.ListOptions{Output: "original"})
	if err != nil {
		return knowledge.Directories{}, sdkError(err)
	}
	entries, err := decodeResult[[]struct {
		knowledge.Directory
		IsDir  bool   `json:"isDir"`
		Access string `json:"access"`
	}](found)
	if err != nil {
		return knowledge.Directories{}, err
	}
	result := knowledge.Directories{Current: dir.value, Entries: make([]knowledge.Directory, 0)}
	if u, err := url.Parse(dir.value); err == nil {
		above := path.Dir(strings.TrimRight(u.Path, "/"))
		if above != "." {
			u.Path, u.RawPath = above, ""
			if scope, ok := parseResourceURI(u.String()); ok && scope.path != dir.path {
				result.Parent = scope.value
			}
		}
	}
	for _, entry := range entries {
		uri, ok := parseResourceURI(entry.URI)
		if entry.IsDir && entry.Access != "denied" && ok && path.Dir(uri.path) == dir.path {
			result.Entries = append(result.Entries, entry.Directory)
		}
	}
	return result, nil
}
