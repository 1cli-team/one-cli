package execution

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issueworkflow"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func (r Runner) codex() issueworkflow.Codex {
	one, _ := exec.LookPath(r.one)
	return issueworkflow.Codex{Executable: "codex", One: one, StateDir: r.root, ReadRoots: r.readRoots, Knowledge: r.knowledge}
}

func (r Runner) models(ctx context.Context) *api.CodexModelCatalog {
	if _, err := exec.LookPath("codex"); err != nil {
		return nil
	}
	catalog, err := r.codex().CodexModels(ctx)
	if err != nil {
		return nil
	}
	return &catalog
}

func supportsModel(catalog api.CodexModelCatalog, options api.CodexOptions) bool {
	if options == (api.CodexOptions{}) {
		return true
	}
	model := options.Model
	if model == "" {
		model = catalog.DefaultModel
	}
	if model == "" {
		return true
	}
	for _, item := range catalog.Models {
		if item.ID == model {
			return options.Effort == "" || slices.Contains(item.Efforts, options.Effort)
		}
	}
	return false
}

func (r Runner) runCodex(ctx context.Context, job assignment, root, commit string, logs io.Writer) api.ExecutionResult {
	common, err := gitOutput(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return failed(err.Error())
	}
	codex := r.codex()
	codex.ChinaMirror = job.ChinaMirror
	if r.reportProgress != nil {
		codex.OnSettings = func(settings api.CodexOptions) {
			if err := r.reportProgress(ctx, api.ExecutionResult{EffectiveCodex: settings}); err != nil {
				slog.Warn(i18n.T("gateway.worker_contact"), "error", err)
			}
		}
	}
	prompt, err := codex.RunPrompt(ctx, root, common, job.Input.Prompt, job.Input.Codex, logs)
	result := api.ExecutionResult{
		ThreadID: prompt.ThreadID, Response: prompt.Response, OpenViking: prompt.OpenViking, EffectiveCodex: prompt.EffectiveCodex,
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if job.Input.PublishRef {
		result.Ref, result.CommitSHA, err = r.publish(ctx, job, root, commit)
		if err != nil {
			result.Error = err.Error()
		}
	}
	return result
}

func (r Runner) publish(ctx context.Context, job assignment, root, base string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	origin, err := r.git.Origin(ctx, root)
	if err != nil {
		return "", "", err
	}
	if !job.Input.Repository.Matches(gitmodule.ParseRepositoryRef(origin.PushURL)) {
		return "", "", i18n.Errorf("gateway.execution_push_origin")
	}
	if _, err := gitOutput(ctx, root, "add", "-A", "--", "."); err != nil {
		return "", "", err
	}
	status, err := gitOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		if _, err := gitOutput(ctx, root, "-c", "core.hooksPath="+os.DevNull, "commit", "-m", "One Gateway task "+job.ID); err != nil {
			return "", "", err
		}
	}
	sha, err := gitOutput(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", err
	}
	if _, err := gitOutput(ctx, root, "merge-base", "--is-ancestor", base, sha); err != nil {
		return "", "", err
	}
	// A separate branch per attempt keeps a delayed push from affecting a retry.
	ref := "refs/heads/one/gateway/" + job.ID + "-" + strings.ToLower(job.Attempt)
	if _, err := gitOutput(ctx, root, "push", "origin", sha+":"+ref); err != nil {
		return ref, sha, err
	}
	return ref, sha, nil
}
