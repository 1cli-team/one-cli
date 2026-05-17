package infisical

// Sync is the workspace-scoped Sync entry point for the Infisical
// backend. It is a no-op — first-time Infisical configuration happens
// through create-time/lazy auto-bind, not as a side effect of selecting
// infisical as the env backend.
func Sync() error { return nil }
