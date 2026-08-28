// Package cicmd contributes the optional `one ci` command family.
//
// CI is deliberately not enabled by workspace creation, project addition, or
// deployment. This command is the sole user-facing entry point that writes or
// removes CI workflow files. The generated workflow itself is the state: no CI
// selection is added to one.manifest.json.
package cicmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

func init() {
	cliexts.Register("ci", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newCICmd()}
}

const githubActionsProviderID = pkgci.DefaultProviderID

type selectionFlags struct {
	project string
}

type enableFlags struct {
	selectionFlags
	provider string
}

type disableFlags struct {
	selectionFlags
	yes bool
}

func newCICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ci",
		Long:    i18n.T("ci.tip"),
		Example: "  one ci\n  one ci enable web\n  one ci sync\n  one ci disable web",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStatus()
		},
	}
	cmd.AddCommand(newEnableCmd(), newSyncCmd(), newDisableCmd())
	i18n.MarkShort(cmd, "ci.short")
	i18n.MarkLong(cmd, "ci.tip")
	return cmd
}

func newEnableCmd() *cobra.Command {
	flags := &enableFlags{}
	cmd := &cobra.Command{
		Use:     "enable [project]",
		Long:    i18n.T("ci.enable.tip"),
		Example: "  one ci enable\n  one ci enable web\n  one ci enable web --provider ci/github-actions",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			selector, err := resolveSelector(args, flags.project)
			if err != nil {
				return err
			}
			return runEnable(selector, flags.provider)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("ci.flag.project"))
	cmd.Flags().StringVar(&flags.provider, "provider", "", i18n.T("ci.flag.provider"))
	i18n.MarkFlagUsage(cmd, "project", "ci.flag.project")
	i18n.MarkFlagUsage(cmd, "provider", "ci.flag.provider")
	helpui.MarkAdvanced(cmd, "project", "provider")
	i18n.MarkShort(cmd, "ci.enable.short")
	i18n.MarkLong(cmd, "ci.enable.tip")
	return cmd
}

func newSyncCmd() *cobra.Command {
	flags := &selectionFlags{}
	cmd := &cobra.Command{
		Use:     "sync [project]",
		Long:    i18n.T("ci.sync.tip"),
		Example: "  one ci sync\n  one ci sync web",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			selector, err := resolveSelector(args, flags.project)
			if err != nil {
				return err
			}
			return runSync(selector)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("ci.flag.project"))
	i18n.MarkFlagUsage(cmd, "project", "ci.flag.project")
	helpui.MarkAdvanced(cmd, "project")
	i18n.MarkShort(cmd, "ci.sync.short")
	i18n.MarkLong(cmd, "ci.sync.tip")
	return cmd
}

func newDisableCmd() *cobra.Command {
	flags := &disableFlags{}
	cmd := &cobra.Command{
		Use:     "disable [project]",
		Long:    i18n.T("ci.disable.tip"),
		Example: "  one ci disable web\n  one ci disable --yes",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			selector, err := resolveSelector(args, flags.project)
			if err != nil {
				return err
			}
			return runDisable(selector, flags.yes)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("ci.flag.project"))
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, i18n.T("ci.flag.yes"))
	i18n.MarkFlagUsage(cmd, "project", "ci.flag.project")
	i18n.MarkFlagUsage(cmd, "yes", "ci.flag.yes")
	helpui.MarkAdvanced(cmd, "project")
	i18n.MarkShort(cmd, "ci.disable.short")
	i18n.MarkLong(cmd, "ci.disable.tip")
	return cmd
}

func runStatus() error {
	root, manifest, err := loadWorkspace()
	if err != nil {
		return err
	}
	result, err := buildStatus(root, manifest)
	if err != nil {
		return err
	}
	output.Emit(result)
	return nil
}

func runEnable(selector, providerFlag string) error {
	root, manifest, err := loadWorkspace()
	if err != nil {
		return err
	}
	projects, err := selectProjects(root, manifest, selector, true)
	if err != nil {
		return err
	}
	providerID, err := resolveProviderID(providerFlag)
	if err != nil {
		return err
	}

	items := make([]actionProject, 0, len(projects))
	for _, project := range projects {
		res, syncErr := syncProject(root, project, providerID)
		if syncErr != nil {
			return syncErr
		}
		status := "updated"
		if res.Created {
			status = "created"
		}
		items = append(items, actionProject{
			Name:         project.Name,
			Status:       status,
			Enabled:      true,
			Provider:     providerID,
			WorkflowPath: relativeWorkflowPath(root, res.WorkflowPath),
			Changed:      true,
		})
	}
	output.Emit(&actionResult{
		Schema:      "one-cli/ci-enable/v1",
		Action:      "enable",
		Provider:    providerID,
		Projects:    items,
		NextCommand: "git status",
	})
	return nil
}

func runSync(selector string) error {
	root, manifest, err := loadWorkspace()
	if err != nil {
		return err
	}
	projects, err := selectProjects(root, manifest, selector, true)
	if err != nil {
		return err
	}

	enabled := make([]workspace.Project, 0, len(projects))
	for _, project := range projects {
		workflowPath := ci.ResolvePathFor(root, project.TargetDir, githubActionsProviderID)
		exists, existsErr := workflowExists(workflowPath)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			enabled = append(enabled, project)
		}
	}
	if len(enabled) == 0 {
		return ciNotEnabledError(selector)
	}

	items := make([]actionProject, 0, len(enabled))
	for _, project := range enabled {
		res, syncErr := syncProject(root, project, githubActionsProviderID)
		if syncErr != nil {
			return syncErr
		}
		items = append(items, actionProject{
			Name:         project.Name,
			Status:       "updated",
			Enabled:      true,
			Provider:     githubActionsProviderID,
			WorkflowPath: relativeWorkflowPath(root, res.WorkflowPath),
			Changed:      true,
		})
	}
	output.Emit(&actionResult{
		Schema:      "one-cli/ci-sync/v1",
		Action:      "sync",
		Provider:    githubActionsProviderID,
		Projects:    items,
		NextCommand: "git status",
	})
	return nil
}

func runDisable(selector string, yes bool) error {
	root, manifest, err := loadWorkspace()
	if err != nil {
		return err
	}
	projects, err := selectProjects(root, manifest, selector, false)
	if err != nil {
		return err
	}

	enabledCount := 0
	for _, project := range projects {
		workflowPath := ci.ResolvePathFor(root, project.TargetDir, githubActionsProviderID)
		exists, existsErr := workflowExists(workflowPath)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			enabledCount++
		}
	}
	if enabledCount > 0 && output.CanPrompt() && !yes {
		confirmed, promptErr := prompt.Confirm(
			i18n.Tf("ci.disable.confirm", enabledCount),
			false,
			i18n.T("ci.disable.confirm_yes"),
			i18n.T("ci.disable.confirm_no"),
		)
		if promptErr != nil {
			return promptErr
		}
		if !confirmed {
			return cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("common.cancelled")).WithExit0()
		}
	}

	items := make([]actionProject, 0, len(projects))
	changed := false
	for _, project := range projects {
		workflowPath := ci.ResolvePathFor(root, project.TargetDir, githubActionsProviderID)
		removed, removeErr := removeWorkflow(workflowPath)
		if removeErr != nil {
			return cliErrors.New(cliErrors.ONE_CLI_ERROR,
				fmt.Sprintf("remove CI workflow for %s: %v", project.Name, removeErr)).
				WithContext(map[string]any{
					"project":       project.Name,
					"workflow_path": relativeWorkflowPath(root, workflowPath),
				})
		}
		status := "unchanged"
		if removed {
			status = "removed"
			changed = true
		}
		items = append(items, actionProject{
			Name:         project.Name,
			Status:       status,
			Enabled:      false,
			WorkflowPath: relativeWorkflowPath(root, workflowPath),
			Changed:      removed,
		})
	}
	next := ""
	if changed {
		next = "git status"
	}
	output.Emit(&actionResult{
		Schema:      "one-cli/ci-disable/v1",
		Action:      "disable",
		Provider:    githubActionsProviderID,
		Projects:    items,
		NextCommand: next,
	})
	return nil
}

func loadWorkspace() (string, *workspace.Manifest, error) {
	root, err := workspace.ResolveProjectRoot("")
	if err != nil {
		return "", nil, err
	}
	if !workspace.HasManifest(root) {
		return "", nil, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			i18n.T("ci.error.not_workspace")).
			WithContext(map[string]any{
				"workspace_root": root,
				"manifest_path":  workspace.ManifestPath(root),
			})
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		return "", nil, err
	}
	return root, manifest, nil
}

func resolveSelector(args []string, legacy string) (string, error) {
	positional := ""
	if len(args) > 0 {
		positional = strings.TrimSpace(args[0])
	}
	legacy = strings.TrimSpace(legacy)
	if positional != "" && legacy != "" && positional != legacy {
		return "", cliErrors.New(cliErrors.ONE_CLI_ERROR,
			i18n.T("ci.error.project_conflict")).
			WithContext(map[string]any{
				"positional_project": positional,
				"flag_project":       legacy,
			})
	}
	if positional != "" {
		return positional, nil
	}
	return legacy, nil
}

func selectProjects(root string, manifest *workspace.Manifest, selector string, requireNonEmpty bool) ([]workspace.Project, error) {
	if strings.TrimSpace(selector) != "" {
		project, err := workspace.ResolveProjectFromSelector(root, selector)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
				i18n.Tf("ci.error.project_not_found", selector)).
				WithContext(map[string]any{
					"selector":           selector,
					"available_projects": workspace.ProjectNames(manifest),
				})
		}
		return []workspace.Project{*project}, nil
	}

	projects := make([]workspace.Project, 0, len(manifest.Projects))
	for _, project := range manifest.Projects {
		projects = append(projects, workspace.Project{
			Name:           project.Name,
			TargetDir:      filepath.Join(root, filepath.FromSlash(project.RelativeDir)),
			RelativeDir:    project.RelativeDir,
			Toolchain:      project.Toolchain,
			PackageManager: project.PackageManager,
			TemplateID:     project.TemplateID,
		})
	}
	if requireNonEmpty && len(projects) == 0 {
		return nil, cliErrors.New(cliErrors.MANIFEST_MISSING_OR_EMPTY,
			i18n.T("ci.error.no_projects"))
	}
	return projects, nil
}

func resolveProviderID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "github-actions" {
		raw = githubActionsProviderID
	}
	if pkgci.Lookup(raw) != nil {
		return raw, nil
	}
	available := make([]string, 0, len(pkgci.Providers()))
	for _, provider := range pkgci.Providers() {
		available = append(available, provider.ID())
	}
	return "", cliErrors.New(cliErrors.CI_PROVIDER_UNKNOWN,
		i18n.Tf("ci.error.provider_unknown", raw)).
		WithContext(map[string]any{
			"provider":            raw,
			"available_providers": available,
		}).
		WithRemediation(output.Remediation{
			Action:  "use-supported-provider",
			Hint:    i18n.T("ci.error.provider_hint"),
			Command: "one ci enable <project> --provider ci/github-actions",
		})
}

func syncProject(root string, project workspace.Project, providerID string) (ci.SyncResult, error) {
	res, err := ci.Sync(ci.SyncOptions{
		ProjectRoot:    root,
		TargetDir:      project.TargetDir,
		ProjectName:    project.Name,
		Toolchain:      toolchain.Toolchain(project.Toolchain),
		PackageManager: toolchain.PackageManager(project.PackageManager),
		ProviderID:     providerID,
	})
	if err == nil {
		return res, nil
	}
	return ci.SyncResult{}, cliErrors.New(cliErrors.CI_RENDER_FAILED,
		fmt.Sprintf("render CI workflow for %s: %v", project.Name, err)).
		WithContext(map[string]any{
			"project":  project.Name,
			"provider": providerID,
		})
}

func workflowExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, cliErrors.New(cliErrors.CI_RENDER_FAILED,
				fmt.Sprintf("CI workflow path is a directory: %s", path))
		}
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func removeWorkflow(path string) (bool, error) {
	exists, err := workflowExists(path)
	if err != nil || !exists {
		return false, err
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

func ciNotEnabledError(selector string) error {
	command := "one ci enable"
	message := i18n.T("ci.error.not_enabled")
	context := map[string]any{}
	if selector != "" {
		command += " " + selector
		message = i18n.Tf("ci.error.project_not_enabled", selector)
		context["project"] = selector
	}
	return cliErrors.New(cliErrors.CI_NOT_ENABLED, message).
		WithContext(context).
		WithRemediation(output.Remediation{
			Action:  "enable-ci",
			Hint:    i18n.T("ci.error.enable_hint"),
			Command: command,
		})
}

func relativeWorkflowPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

type statusResult struct {
	Schema      string          `json:"schema"`
	Configured  bool            `json:"configured"`
	Provider    string          `json:"provider,omitempty"`
	Projects    []projectStatus `json:"projects"`
	NextCommand string          `json:"next_command"`
}

type projectStatus struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider,omitempty"`
	WorkflowPath string `json:"workflow_path"`
}

func buildStatus(root string, manifest *workspace.Manifest) (*statusResult, error) {
	result := &statusResult{
		Schema:      "one-cli/ci-status/v1",
		Projects:    make([]projectStatus, 0, len(manifest.Projects)),
		NextCommand: "one add",
	}
	for _, project := range manifest.Projects {
		targetDir := filepath.Join(root, filepath.FromSlash(project.RelativeDir))
		workflowPath := ci.ResolvePathFor(root, targetDir, githubActionsProviderID)
		enabled, err := workflowExists(workflowPath)
		if err != nil {
			return nil, err
		}
		item := projectStatus{
			Name:         project.Name,
			Enabled:      enabled,
			WorkflowPath: relativeWorkflowPath(root, workflowPath),
		}
		if enabled {
			item.Provider = githubActionsProviderID
			result.Configured = true
			result.Provider = githubActionsProviderID
		}
		result.Projects = append(result.Projects, item)
	}
	for _, project := range result.Projects {
		if !project.Enabled {
			result.NextCommand = "one ci enable " + project.Name
			return result, nil
		}
	}
	if len(result.Projects) > 0 {
		result.NextCommand = "one ci sync"
	}
	return result, nil
}

func (r *statusResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	state := i18n.T("ci.status.not_configured")
	if r.Configured {
		state = providerLabel(r.Provider)
	}
	fmt.Fprintf(w, "%s%s\n", i18n.T("ci.status.heading"), state)
	fmt.Fprintln(w, i18n.T("ci.status.projects"))
	if len(r.Projects) == 0 {
		fmt.Fprintln(w, i18n.T("ci.status.no_projects"))
	}
	for _, project := range r.Projects {
		state := i18n.T("ci.status.disabled")
		path := ""
		if project.Enabled {
			state = i18n.T("ci.status.enabled")
			path = "  " + project.WorkflowPath
		}
		fmt.Fprintf(w, "  %-16s %s%s\n", project.Name, state, path)
	}
	if r.NextCommand != "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s%s\n", i18n.T("ci.status.next"), r.NextCommand)
	}
}

type actionResult struct {
	Schema      string          `json:"schema"`
	Action      string          `json:"action"`
	Provider    string          `json:"provider"`
	Projects    []actionProject `json:"projects"`
	NextCommand string          `json:"next_command,omitempty"`
}

type actionProject struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider,omitempty"`
	WorkflowPath string `json:"workflow_path"`
	Changed      bool   `json:"changed"`
}

func (r *actionResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	switch r.Action {
	case "enable":
		fmt.Fprintf(w, i18n.T("ci.enable.success")+"\n", len(r.Projects))
		fmt.Fprintf(w, "%s%s\n", i18n.T("ci.action.service"), providerLabel(r.Provider))
	case "sync":
		fmt.Fprintf(w, i18n.T("ci.sync.success")+"\n", len(r.Projects))
	case "disable":
		changed := 0
		for _, project := range r.Projects {
			if project.Changed {
				changed++
			}
		}
		fmt.Fprintf(w, i18n.T("ci.disable.success")+"\n", changed)
	}
	for _, project := range r.Projects {
		label := i18n.T("ci.action.unchanged")
		if project.Changed {
			label = i18n.T("ci.action." + project.Status)
		}
		fmt.Fprintf(w, "  %s  %s  %s\n", project.Name, label, project.WorkflowPath)
	}
	if r.NextCommand != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, i18n.T("common.next_steps"))
		fmt.Fprintf(w, "  %s\n", r.NextCommand)
	}
}

func providerLabel(provider string) string {
	if provider == githubActionsProviderID {
		return "GitHub Actions"
	}
	return provider
}
