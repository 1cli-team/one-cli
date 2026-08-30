# One CLI Kernel

`packages/kernel` is the reusable Go runtime shared by One CLI processes. It
adapts Cordis context and effect ownership to static, constructor-injected Go.
It does not provide dynamic plugins, service lookup, hot reload, or an event
bus.

The module follows the applicable parts of
[`golang-standards/project-layout`](https://github.com/golang-standards/project-layout):

```text
packages/kernel/
  docs/             design documentation
  internal/         private runtime implementation
  pkg/kernel/       public context and lifecycle interface
  go.mod            independent Go module
```

Directories intended for applications, deployments, or web assets are omitted
because Kernel is a library module. Add them only when a real Kernel-owned use
case exists.
