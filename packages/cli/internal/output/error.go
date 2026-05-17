package output

// Remediation is one suggested recovery step for an error. Shape is
// part of the public JSON error envelope.
type Remediation struct {
	// Action is a machine-readable identifier (e.g. "use-different-name").
	Action string `json:"action"`
	// Hint is a human-readable suggestion (Chinese OK).
	Hint string `json:"hint,omitempty"`
	// Command is an optional concrete command the user/agent can run.
	Command string `json:"command,omitempty"`
	// Destructive marks the action as data-losing.
	Destructive bool `json:"destructive,omitempty"`
}

// Error is the structured CLI error type. It mirrors OneCliError in TS:
// the public envelope is `{ schema: "one-cli/error/v1", error: {...} }`.
type Error struct {
	Code        string         `json:"-"`
	Message     string         `json:"-"`
	Context     map[string]any `json:"-"`
	Remediation []Remediation  `json:"-"`
	// Exit0 marks an error that should still emit the JSON envelope but
	// produce a zero exit status — used for user-cancelled prompts where
	// "we communicated something via stderr but the user's choice was
	// valid". Defaults to false (exit 1).
	Exit0 bool `json:"-"`
}

// NewError constructs an error with no remediation. Most callers should use
// the helpers in internal/errors which fill in default remediation from the
// central code registry.
func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithContext returns a new error with structured context attached.
func (e *Error) WithContext(ctx map[string]any) *Error {
	out := *e
	out.Context = ctx
	return &out
}

// WithRemediation returns a new error with remediation steps attached.
func (e *Error) WithRemediation(steps ...Remediation) *Error {
	out := *e
	out.Remediation = append([]Remediation{}, steps...)
	return &out
}

// WithExit0 marks the error as a graceful-exit signal. The envelope still
// flows through EmitError so the schema stays consistent, but main.go
// returns exit code 0. Reserved for cooperatively-cancelled flows
// (Ctrl-C in a prompt, user-chosen "cancel" option, etc.).
func (e *Error) WithExit0() *Error {
	out := *e
	out.Exit0 = true
	return &out
}

// Error implements the error interface so *Error can flow through the usual
// error-handling channels.
func (e *Error) Error() string { return e.Message }

// ErrorCode exposes the machine-readable code for callers that want to
// branch on it without depending on the *Error concrete type.
func (e *Error) ErrorCode() string { return e.Code }

// envelope produces the JSON-shape envelope, matching the TS contract:
// context defaults to {}, remediation defaults to []. Both fields are always
// present (no omitempty) because agents pattern-match on shape stability.
func (e *Error) envelope() errorEnvelope {
	ctx := e.Context
	if ctx == nil {
		ctx = map[string]any{}
	}
	rem := e.Remediation
	if rem == nil {
		rem = []Remediation{}
	}
	return errorEnvelope{
		Schema: "one-cli/error/v1",
		Error: errorBody{
			Code:        e.Code,
			Message:     e.Message,
			Context:     ctx,
			Remediation: rem,
		},
	}
}

type errorEnvelope struct {
	Schema string    `json:"schema"`
	Error  errorBody `json:"error"`
}

type errorBody struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Context     map[string]any `json:"context"`
	Remediation []Remediation  `json:"remediation"`
}
