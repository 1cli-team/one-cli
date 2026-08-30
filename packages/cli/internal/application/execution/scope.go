package execution

import (
	"context"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/kernel/pkg/kernel"
)

type ScopePatch struct {
	WorkspaceRoot string
	Project       string
	Environment   string
	Backend       catalog.BackendID
}

// ExecutionScope is immutable command state. Derive returns a copy sharing
// only the lifecycle, so child operations cannot mutate their parent's view.
type Scope struct {
	runtime          kernel.Context
	workingDirectory string
	workspaceRoot    string
	project          string
	environment      string
	backend          catalog.BackendID
}

func NewScope(ctx context.Context, workingDirectory string) Scope {
	return Scope{
		runtime: kernel.NewContext(ctx), workingDirectory: workingDirectory,
	}
}

func (s Scope) Derive(patch ScopePatch) Scope {
	child := s
	if patch.WorkspaceRoot != "" {
		child.workspaceRoot = patch.WorkspaceRoot
	}
	if patch.Project != "" {
		child.project = patch.Project
	}
	if patch.Environment != "" {
		child.environment = patch.Environment
	}
	if patch.Backend.String() != "" {
		child.backend = patch.Backend
	}
	return child
}

func (s Scope) Context() context.Context        { return s.runtime.Context() }
func (s Scope) WorkingDirectory() string        { return s.workingDirectory }
func (s Scope) WorkspaceRoot() string           { return s.workspaceRoot }
func (s Scope) Project() string                 { return s.project }
func (s Scope) Environment() string             { return s.environment }
func (s Scope) Backend() catalog.BackendID      { return s.backend }
func (s Scope) Close(ctx context.Context) error { return s.runtime.Close(ctx) }

type executionScopeContextKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, executionScopeContextKey{}, scope)
}

func FromContext(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}
	scope, ok := ctx.Value(executionScopeContextKey{}).(Scope)
	return scope, ok
}
