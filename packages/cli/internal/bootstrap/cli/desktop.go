package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/template"
	creation "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/creation"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/installation"
)

// DesktopCommand is a narrow native bridge to local Core operations. Data
// registration still uses the same management client as ordinary CLI commands.
func DesktopCommand(args []string, version string) (bool, error) {
	if len(args) == 0 || !strings.HasPrefix(args[0], "__desktop-") {
		return false, nil
	}
	if args[0] == "__desktop-identity" {
		return false, nil
	}
	if _, err := bootstrap.InitializeStorage(); err != nil {
		return true, err
	}
	ctx := context.Background()
	client := oneclient.New()
	var result any
	switch args[0] {
	case "__desktop-remote-token":
		var in struct {
			URL   string `json:"url"`
			Token string `json:"token"`
			Save  bool   `json:"save"`
		}
		if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&in); err != nil {
			return true, err
		}
		token, err := oneclient.RemoteToken(in.URL, in.Token, in.Save)
		if err != nil {
			return true, err
		}
		result = map[string]string{"token": token}
	case "__desktop-observe":
		var input struct {
			Path      string `json:"path"`
			ServerURL string `json:"serverURL"`
			Local     bool   `json:"local"`
		}
		if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&input); err != nil {
			return true, err
		}
		root, err := filepath.Abs(input.Path)
		if err != nil {
			return true, err
		}
		if err = client.BindAddress(ctx, root, input.ServerURL, input.Local); err != nil {
			return true, err
		}
		_, snapshot, err := client.Register(ctx, root, "")
		if err != nil {
			return true, err
		}
		result = snapshot
	case "__desktop-install-status":
		target, err := installation.DefaultTarget()
		if err != nil {
			return true, err
		}
		result, err = installation.Inspect(ctx, target, version)
		if err != nil {
			return true, err
		}
	case "__desktop-create-project":
		var input struct {
			WorkspaceID string `json:"workspaceId"`
			ServerURL   string `json:"serverURL"`
			Name        string `json:"name"`
			TemplateID  string `json:"templateId"`
			SkipSkills  bool   `json:"skipSkills"`
		}
		if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&input); err != nil {
			return true, err
		}
		root, err := client.LocalRootFor(input.WorkspaceID, input.ServerURL)
		if err != nil {
			return true, err
		}
		conn, err := client.ConnectionFor(ctx, root, "")
		if err != nil {
			return true, err
		}
		target, err := oneclient.NormalizeURL(input.ServerURL)
		if err != nil {
			return true, err
		}
		if conn.URL != target {
			return true, i18n.Errorf("server.checkout_mismatch")
		}
		registry, err := template.Fetch(ctx, "")
		if err != nil {
			return true, err
		}
		selected, err := registry.Resolve(input.TemplateID)
		if err != nil {
			return true, err
		}
		service := bootstrap.Compose(client.Observe, oneclient.Loader{Client: client}).Creation
		created, err := service.AddProject(ctx, root, creation.ProjectInput{Template: selected, Name: input.Name, SkipSkills: input.SkipSkills})
		if err != nil {
			return true, err
		}
		var project func(creation.ProjectResult) map[string]any
		project = func(p creation.ProjectResult) map[string]any {
			relative, _ := filepath.Rel(root, p.TargetPath)
			children := []map[string]any{}
			for _, child := range p.Projects {
				children = append(children, project(child))
			}
			return map[string]any{"name": p.Name, "relativeDir": filepath.ToSlash(relative), "templateId": p.TemplateID, "warnings": p.Warnings, "projects": children}
		}
		result = project(created.Project)
	case "__desktop-local-path":
		if len(args) != 2 {
			return true, i18n.Errorf("workspace.manifest_required")
		}
		root, err := client.LocalRoot(args[1])
		if err != nil {
			return true, err
		}
		result = map[string]string{"root": root}
	default:
		return true, i18n.Errorf("server.usage")
	}
	return true, json.NewEncoder(os.Stdout).Encode(result)
}
