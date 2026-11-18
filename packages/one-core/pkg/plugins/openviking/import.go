package openviking

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

var taskIDPattern = regexp.MustCompile("^[A-Za-z0-9_-]{1,128}$")

func (p *Plugin) Import(ctx context.Context, workspaceID, name string, source io.Reader) (knowledge.Import, error) {
	saved, root, err := p.bindingFor(ctx, workspaceID)
	if err != nil {
		return knowledge.Import{}, err
	}
	if strings.TrimSpace(name) == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\") || strings.ContainsFunc(name, unicode.IsControl) {
		return knowledge.Import{}, invalidImportFile()
	}

	dir, err := os.MkdirTemp("", "one-openviking-")
	if err != nil {
		return knowledge.Import{}, err
	}
	defer os.RemoveAll(dir)
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return knowledge.Import{}, err
	}
	size, copyErr := io.Copy(file, io.LimitReader(source, knowledge.MaxImportSize+1))
	closeErr := file.Close()
	if size > knowledge.MaxImportSize || size == 0 || copyErr != nil || closeErr != nil {
		return knowledge.Import{}, invalidImportFile()
	}
	client, err := sdk(saved, 3*time.Minute)
	if err != nil {
		return knowledge.Import{}, err
	}
	response, err := client.AddResource(ctx, file.Name(), &ov.AddResourceOptions{Parent: root.value})
	if err != nil {
		return knowledge.Import{}, sdkError(err)
	}
	submitted, err := decodeResult[struct {
		TaskID string `json:"task_id"`
		URI    string `json:"root_uri"`
		Status string `json:"status"`
	}](response)
	if err != nil {
		return knowledge.Import{}, err
	}
	if !taskIDPattern.MatchString(submitted.TaskID) ||
		(submitted.Status != "success" && submitted.Status != "accepted") {
		return knowledge.Import{}, invalidResponse()
	}
	if submitted.URI != "" && !containsImportedResource(root, submitted.URI) {
		return knowledge.Import{}, outsideScope()
	}
	return knowledge.Import{TaskID: submitted.TaskID, URI: submitted.URI, Status: "pending"}, nil
}

func (p *Plugin) ImportStatus(ctx context.Context, workspaceID, taskID string) (knowledge.Import, error) {
	if !taskIDPattern.MatchString(taskID) {
		return knowledge.Import{}, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_task_id"))
	}
	saved, root, err := p.bindingFor(ctx, workspaceID)
	if err != nil {
		return knowledge.Import{}, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Import{}, err
	}
	response, err := client.GetTask(ctx, taskID)
	if err != nil {
		return knowledge.Import{}, sdkError(err)
	}
	task, err := decodeResult[struct {
		ID         string `json:"task_id"`
		Type       string `json:"task_type"`
		Status     string `json:"status"`
		ResourceID string `json:"resource_id"`
		Error      string `json:"error"`
		Result     struct {
			RootURI string `json:"root_uri"`
		} `json:"result"`
	}](response)
	if err != nil {
		return knowledge.Import{}, err
	}
	if task.ID != taskID || task.Type != "add_resource" {
		return knowledge.Import{}, outsideScope()
	}
	result := knowledge.Import{TaskID: taskID, URI: task.ResourceID, Status: task.Status}
	if task.Result.RootURI != "" {
		result.URI = task.Result.RootURI
	}
	if result.URI == "" {
		// A pending server-side parser may not have determined the URI yet.
		if task.Status == "pending" || task.Status == "running" {
			return knowledge.Import{TaskID: taskID, Status: "pending"}, nil
		}
		return knowledge.Import{}, outsideScope()
	}
	if !containsImportedResource(root, result.URI) ||
		(task.ResourceID != "" && !containsImportedResource(root, task.ResourceID)) {
		return knowledge.Import{}, outsideScope()
	}
	switch result.Status {
	case "pending", "running", "cancelling", "completed", "cancelled":
	case "failed":
		result.Error = task.Error
		if saved.APIKey != "" {
			result.Error = strings.ReplaceAll(result.Error, saved.APIKey, "[redacted]")
		}
	default:
		return knowledge.Import{}, invalidResponse()
	}
	return result, nil
}

func containsImportedResource(root resourceURI, value string) bool {
	uri, ok := parseResourceURI(value)
	return ok && uri.path != root.path && root.contains(uri)
}

func invalidImportFile() error {
	return cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_import_file"))
}

func invalidResponse() error {
	return cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.T("openviking.invalid_response"))
}

func outsideScope() error {
	return cliErrors.New(cliErrors.OPENVIKING_ACCESS_DENIED, i18n.T("openviking.outside_scope"))
}
