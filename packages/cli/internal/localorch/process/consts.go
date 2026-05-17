package processorch

// Stable JSON envelope schema string for the dev start operation.
const SchemaStart = "one-cli/dev-start/v1"

// builtinRunnerID is the sentinel value returned in StartResult.Runner.
// One CLI's supervisor is now the only runner — external Procfile
// runners (overmind / hivemind / foreman / honcho) are no longer
// probed. The field is kept for forward-compat with consumers that
// switch on the runner name.
const builtinRunnerID = "builtin"
