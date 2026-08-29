# Kernel architecture

Kernel owns process-local execution mechanics, not One CLI product policy.

## Public interface

`pkg/kernel` exposes execution context ownership and lifecycle cleanup. The
interface is intentionally small enough for CLI, daemon, and agent processes to
share without depending on one another.

## Private implementation

`internal/effect` owns synchronization, cleanup ordering, error joining, and
idempotent shutdown.

## Dependency rule

Kernel imports only the Go standard library. It must not know about Workspace,
Project, Environment, Backend, Profile, Template, transports, or adapters.
