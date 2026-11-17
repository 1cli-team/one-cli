// Package git contributes local repository and worktree operations.
package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/flock"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
	registryport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/workspaceregistry"
)

type Plugin struct{}

func New() *Plugin                                     { return &Plugin{} }
func (*Plugin) ID() string                             { return "git" }
func (p *Plugin) Contributions() plugins.Contributions { return plugins.Contributions{Git: p} }

func run(ctx context.Context, root string, args ...string) ([]byte, error) {
	return runWithTimeout(ctx, root, 2*time.Minute, args...)
}

func runWithTimeout(ctx context.Context, root string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := process.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	// An inherited Git context must not redirect an operation to another repo.
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !slices.Contains([]string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_OPTIONAL_LOCKS", "GIT_TERMINAL_PROMPT"}, key) {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, i18n.Errorf("git.command_failed", root, strings.TrimSpace(stderr.String()), err)
	}
	return out, nil
}

func canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	resolved, err = canonical(parent)
	return filepath.Join(resolved, filepath.Base(abs)), err
}

func commonDir(ctx context.Context, root string) (string, error) {
	out, err := run(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return canonical(strings.TrimSuffix(string(out), "\n"))
}

func treeID(common, path string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(common+"\x00"+path)))[:24]
}

func (p *Plugin) InspectWorkspaceRepository(ctx context.Context, root string) (*registryport.GitRepository, error) {
	common, err := commonDir(ctx, root)
	if err != nil {
		return nil, err
	}
	trees, err := list(ctx, root, common)
	if err != nil {
		return nil, err
	}
	result := &registryport.GitRepository{CommonDir: common}
	for _, tree := range trees {
		result.Worktrees = append(result.Worktrees, registryport.GitWorktree{ID: tree.ID, Root: tree.Path, Branch: tree.Branch, Main: tree.Main, Bare: tree.Bare})
	}
	return result, nil
}

func parseWorktrees(data []byte, common string) ([]gitmodule.Worktree, error) {
	trees := []gitmodule.Worktree{}
	var tree *gitmodule.Worktree
	for _, field := range bytes.Split(data, []byte{0}) {
		key, value, _ := strings.Cut(string(field), " ")
		if key == "worktree" {
			path, err := canonical(value)
			if err != nil {
				return nil, err
			}
			trees = append(trees, gitmodule.Worktree{ID: treeID(common, path), Path: path, Main: len(trees) == 0})
			tree = &trees[len(trees)-1]
			continue
		}
		if tree == nil {
			continue
		}
		switch key {
		case "HEAD":
			tree.Head = value
		case "branch":
			tree.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			tree.Bare = true
		case "detached":
			tree.Detached = true
		case "locked":
			tree.Locked = true
			tree.LockReason = value
		case "prunable":
			tree.Prunable = true
		}
	}
	return trees, nil
}

func list(ctx context.Context, root, common string) ([]gitmodule.Worktree, error) {
	out, err := run(ctx, root, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out, common)
}

func (p *Plugin) Inspect(ctx context.Context, root string) (*gitmodule.Repository, error) {
	common, err := commonDir(ctx, root)
	if err != nil {
		return nil, err
	}
	trees, err := list(ctx, root, common)
	if err != nil {
		return nil, err
	}
	for index := range trees {
		tree := &trees[index]
		_, err := os.Stat(tree.Path)
		tree.Missing = errors.Is(err, os.ErrNotExist)
		if tree.Missing || tree.Bare {
			continue
		}
		out, err := run(ctx, tree.Path, "status", "--porcelain=v1", "-z", "--untracked-files=normal")
		if err != nil {
			tree.StatusError = err.Error()
			continue
		}
		tree.Dirty = len(out) > 0
	}
	branches, err := run(ctx, root, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	branchNames := []string{}
	if value := strings.TrimSpace(string(branches)); value != "" {
		branchNames = strings.Split(value, "\n")
	}
	base := root
	if len(trees) > 0 {
		base = trees[0].Path
	}
	remotes, err := readRemotes(ctx, root)
	if err != nil {
		return nil, err
	}
	return &gitmodule.Repository{
		Schema:        "one-cli/git-worktrees/v1",
		CommonDir:     common,
		DefaultParent: filepath.Join(base, ".one", "worktrees"),
		Remotes:       remotes,
		Branches:      branchNames,
		Worktrees:     trees,
	}, nil
}

func remoteURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	if parsed.User != nil {
		if _, password := parsed.User.Password(); password || parsed.Scheme == "http" || parsed.Scheme == "https" {
			parsed.User = nil
		}
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func readRemotes(ctx context.Context, root string) ([]gitmodule.Remote, error) {
	data, err := run(ctx, root, "remote", "-v")
	if err != nil {
		return nil, err
	}
	remotes := []gitmodule.Remote{}
	indexes := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		name, target, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		value, operation, ok := strings.Cut(target, " (")
		if !ok {
			continue
		}
		index, found := indexes[name]
		if !found {
			index = len(remotes)
			indexes[name] = index
			remotes = append(remotes, gitmodule.Remote{Name: name})
		}
		switch operation {
		case "fetch)":
			remotes[index].FetchURL = remoteURL(value)
		case "push)":
			remotes[index].PushURL = remoteURL(value)
		}
	}
	return remotes, nil
}

var sshCloneURL = regexp.MustCompile(`^(?:[A-Za-z0-9._-]+@)?[A-Za-z0-9._-]+:[^:\s][^\s]*$`)

func validateCloneURL(source string) error {
	parsed, err := url.Parse(source)
	if err == nil && parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http" || parsed.Scheme == "ssh") {
		password := false
		if parsed.User != nil {
			_, password = parsed.User.Password()
		}
		if parsed.RawQuery == "" && parsed.Fragment == "" && !password && (parsed.User == nil || parsed.Scheme == "ssh") {
			return nil
		}
	}
	if !strings.Contains(source, "://") && sshCloneURL.MatchString(source) {
		return nil
	}
	return i18n.Errorf("git.clone_url_invalid")
}

func (p *Plugin) Clone(ctx context.Context, source, destination string) (string, error) {
	if err := validateCloneURL(source); err != nil {
		return "", err
	}
	if strings.TrimSpace(destination) == "" {
		return "", i18n.Errorf("git.repository_path_required")
	}
	destination, err := canonical(destination)
	if err != nil {
		return "", err
	}
	release, err := acquire(ctx, destination)
	if err != nil {
		return "", err
	}
	defer release()
	destinationInfo, destinationErr := os.Lstat(destination)
	if destinationErr == nil {
		// A completed clone can be retried to recover its association.
		if info, err := p.Inspect(ctx, destination); err == nil {
			for _, remote := range info.Remotes {
				if remote.Name == "origin" && remote.FetchURL == remoteURL(source) {
					for _, tree := range info.Worktrees {
						if tree.Path == destination {
							return destination, nil
						}
					}
				}
			}
		}
		entries, err := os.ReadDir(destination)
		if err != nil || len(entries) != 0 {
			return "", i18n.Errorf("git.path_in_use", destination)
		}
	} else if !errors.Is(destinationErr, os.ErrNotExist) {
		return "", destinationErr
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(parent, ".one-clone-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	checkout := filepath.Join(staging, "checkout")
	if _, err := runWithTimeout(ctx, parent, 15*time.Minute, "clone", "--", source, checkout); err != nil {
		return "", err
	}
	if _, err := p.Inspect(ctx, checkout); err != nil {
		return "", err
	}
	if err := os.Rename(checkout, destination); err != nil {
		// Windows cannot replace an empty directory with a directory rename.
		// Restrict that fallback to the original empty directory.
		if runtime.GOOS != "windows" || destinationInfo == nil {
			return "", err
		}
		current, statErr := os.Lstat(destination)
		if statErr != nil || !current.IsDir() || !os.SameFile(destinationInfo, current) {
			return "", err
		}
		if err := os.Remove(destination); err != nil {
			return "", err
		}
		if err := os.Rename(checkout, destination); err != nil {
			return "", err
		}
	}
	return destination, nil
}

func acquire(ctx context.Context, common string) (func(), error) {
	dir, err := userdirs.Locks()
	if err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "git-"+treeID(common, common)+".lock"))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, i18n.Errorf("git.busy")
	}
	return func() { _ = lock.Unlock() }, nil
}

func (p *Plugin) Add(ctx context.Context, root string, input gitmodule.AddInput) (*gitmodule.Worktree, error) {
	if input.Path == "" || (input.Detached && (input.NewBranch || input.Branch != "")) || (!input.Detached && input.Branch == "") {
		return nil, i18n.Errorf("git.invalid_add")
	}
	path := input.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := canonical(path)
	if err != nil {
		return nil, err
	}
	common, err := commonDir(ctx, root)
	if err != nil {
		return nil, err
	}
	release, err := acquire(ctx, common)
	if err != nil {
		return nil, err
	}
	defer release()
	if input.Branch != "" {
		if _, err := run(ctx, root, "check-ref-format", "refs/heads/"+input.Branch); err != nil {
			return nil, err
		}
		if strings.HasPrefix(input.Branch, "-") {
			return nil, i18n.Errorf("git.invalid_add")
		}
	}
	args := []string{"worktree", "add"}
	start := input.Branch
	if input.NewBranch || input.Detached {
		start = input.StartPoint
		if start == "" {
			start = "HEAD"
		}
		commit, err := run(ctx, root, "rev-parse", "--verify", "--end-of-options", start+"^{commit}")
		if err != nil {
			return nil, err
		}
		start = strings.TrimSpace(string(commit))
		if input.Detached {
			args = append(args, "--detach")
		} else {
			args = append(args, "-b", input.Branch)
		}
	} else {
		if _, err := run(ctx, root, "show-ref", "--verify", "--", "refs/heads/"+input.Branch); err != nil {
			return nil, err
		}
	}
	trees, err := list(ctx, root, common)
	if err != nil {
		return nil, err
	}
	for _, tree := range trees {
		if tree.Path != path {
			continue
		}
		// A retried create may arrive after Git committed but before the caller
		// received its response. Reuse only an exact requested checkout.
		if !tree.Main && !tree.Prunable && ((input.Detached && tree.Detached && tree.Head == start) || (!input.Detached && tree.Branch == input.Branch)) {
			if _, err := os.Stat(tree.Path); err == nil {
				return &tree, nil
			}
		}
		return nil, i18n.Errorf("git.path_in_use", path)
	}
	args = append(args, "--", path, start)
	if _, err := run(ctx, root, args...); err != nil {
		return nil, err
	}
	trees, err = list(ctx, root, common)
	if err != nil {
		return nil, err
	}
	for _, tree := range trees {
		if tree.Path == path {
			return &tree, nil
		}
	}
	return nil, i18n.Errorf("git.readback_failed", path)
}

func (p *Plugin) Remove(ctx context.Context, root, id string) error {
	common, err := commonDir(ctx, root)
	if err != nil {
		return err
	}
	release, err := acquire(ctx, common)
	if err != nil {
		return err
	}
	defer release()
	trees, err := list(ctx, root, common)
	if err != nil {
		return err
	}
	for _, tree := range trees {
		if tree.ID != id {
			continue
		}
		if tree.Main || tree.Bare || tree.Locked {
			return i18n.Errorf("git.remove_protected", tree.Path)
		}
		if _, err := run(ctx, root, "worktree", "remove", "--", tree.Path); err != nil {
			return err
		}
		trees, err := list(ctx, common, common)
		if err != nil {
			return err
		}
		for _, remaining := range trees {
			if remaining.ID == id {
				return i18n.Errorf("git.readback_failed", tree.Path)
			}
		}
		return nil
	}
	// Removal is idempotent and cannot select a path outside this repository.
	return nil
}

func (p *Plugin) WorkspaceIdentity(root string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	common, err := commonDir(ctx, root)
	if err != nil {
		return "", err
	}
	out, err := run(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top, err := canonical(strings.TrimSuffix(string(out), "\n"))
	if err != nil {
		return "", err
	}
	root, err = canonical(root)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(top, root)
	if err != nil || !filepath.IsLocal(relative) {
		return "", i18n.Errorf("git.workspace_unavailable")
	}
	trees, err := list(ctx, root, common)
	if err != nil {
		return "", err
	}
	for _, tree := range trees {
		if tree.Path == top {
			return common + "\x00" + filepath.ToSlash(relative), nil
		}
	}
	return "", i18n.Errorf("git.workspace_unavailable")
}

var _ plugins.Plugin = (*Plugin)(nil)
var _ gitmodule.Backend = (*Plugin)(nil)
