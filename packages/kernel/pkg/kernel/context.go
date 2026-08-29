package kernel

import "context"

// Context owns the standard Go context and lifecycle for one execution tree.
// Derived contexts may replace cancellation and deadline state while sharing
// the same lifecycle owner.
type Context struct {
	ctx       context.Context
	lifecycle *Lifecycle
}

// NewContext creates a root execution context.
func NewContext(ctx context.Context) Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return Context{ctx: ctx, lifecycle: NewLifecycle()}
}

// Derive creates a child context that shares the parent's lifecycle.
func (c Context) Derive(ctx context.Context) Context {
	if ctx == nil {
		ctx = c.Context()
	}
	lifecycle := c.lifecycle
	if lifecycle == nil {
		lifecycle = NewLifecycle()
	}
	return Context{ctx: ctx, lifecycle: lifecycle}
}

// Context returns the standard Go context carried by this execution context.
func (c Context) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Lifecycle returns the owner of temporary effects for this execution tree.
func (c Context) Lifecycle() *Lifecycle { return c.lifecycle }

// Close releases every effect owned by this execution tree.
func (c Context) Close(ctx context.Context) error {
	if c.lifecycle == nil {
		return nil
	}
	return c.lifecycle.Close(ctx)
}
