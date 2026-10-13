package execution

import (
	"context"

	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
)

type ScopePatch struct {
	WorkspaceRoot string
	Project       string
	Environment   string
	Backend       catalog.BackendID
}

// Scope is immutable command state. Derive copies its fields while retaining
// the caller context, including cancellation, deadlines, and values.
type Scope struct {
	ctx              context.Context
	workingDirectory string
	workspaceRoot    string
	project          string
	environment      string
	backend          catalog.BackendID
}

func NewScope(ctx context.Context, workingDirectory string) Scope {
	if ctx == nil {
		ctx = context.Background()
	}
	return Scope{ctx: ctx, workingDirectory: workingDirectory}
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

func (s Scope) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}
func (s Scope) WorkingDirectory() string   { return s.workingDirectory }
func (s Scope) WorkspaceRoot() string      { return s.workspaceRoot }
func (s Scope) Project() string            { return s.project }
func (s Scope) Environment() string        { return s.environment }
func (s Scope) Backend() catalog.BackendID { return s.backend }

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
