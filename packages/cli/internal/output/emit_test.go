package output

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type renderable struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

func (r *renderable) RenderTTY(w io.Writer) {
	_, _ = w.Write([]byte("hello " + r.Name + "\n"))
}

type plain struct {
	Schema string `json:"schema"`
	Value  int    `json:"value"`
}

func TestEmitJSON_Indented_RoundTrips(t *testing.T) {
	var buf bytes.Buffer
	emitJSON(&buf, &renderable{Schema: "one-cli/test/v1", Name: "world"})
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected trailing newline, got %q", got)
	}
	// Pretty-printed: ≥ 1 indented line + closing brace + trailing newline.
	if !strings.Contains(got, "\n  ") {
		t.Errorf("expected 2-space indent in JSON output, got %q", got)
	}
	var back renderable
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &back); err != nil {
		t.Fatalf("emitted output is not valid JSON: %v (%q)", err, got)
	}
	if back.Schema != "one-cli/test/v1" || back.Name != "world" {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

func TestEmitYAML_HonoursJSONTags_RoundTrips(t *testing.T) {
	var buf bytes.Buffer
	emitYAML(&buf, &renderable{Schema: "one-cli/test/v1", Name: "world"})
	got := buf.String()
	// yaml.v3 doesn't honour json tags directly; we round-trip through JSON
	// so the wire keys still come from the json:"..." tags.
	if !strings.Contains(got, "schema: one-cli/test/v1") {
		t.Errorf("yaml output missing schema field with json-tagged key: %q", got)
	}
	if !strings.Contains(got, "name: world") {
		t.Errorf("yaml output missing name field with json-tagged key: %q", got)
	}
}

func TestEmitTo_MarshalFailure_FallbackEnvelope(t *testing.T) {
	var buf bytes.Buffer
	// channels can't be JSON-encoded; this exercises the synthetic envelope fallback.
	emitTo(&buf, make(chan int))
	got := buf.String()
	if !strings.Contains(got, `"schema":"one-cli/error/v1"`) {
		t.Errorf("fallback envelope missing schema: %q", got)
	}
	if !strings.Contains(got, "OUTPUT_MARSHAL_FAILED") {
		t.Errorf("fallback envelope missing code: %q", got)
	}
}

func TestSetMode_IsJSON_Forced(t *testing.T) {
	t.Cleanup(func() { SetMode(ModeAuto) })

	SetMode(ModeJSON)
	if !IsJSON() {
		t.Errorf("SetMode(ModeJSON): IsJSON() = false, want true")
	}
	if IsTTY() {
		t.Errorf("SetMode(ModeJSON): IsTTY() = true, want false")
	}

	SetMode(ModeTTY)
	if IsJSON() {
		t.Errorf("SetMode(ModeTTY): IsJSON() = true, want false")
	}
	if !IsTTY() {
		t.Errorf("SetMode(ModeTTY): IsTTY() = false, want true")
	}
}

func TestEmitError_Envelope_JSON(t *testing.T) {
	t.Cleanup(func() { SetMode(ModeAuto) })
	SetMode(ModeJSON)

	var buf bytes.Buffer
	err := NewError("MY_CODE", "boom").
		WithContext(map[string]any{"k": "v"}).
		WithRemediation(Remediation{Action: "retry", Hint: "try again"})
	emitTo(&buf, err.envelope())

	var got struct {
		Schema string `json:"schema"`
		Error  struct {
			Code        string         `json:"code"`
			Message     string         `json:"message"`
			Context     map[string]any `json:"context"`
			Remediation []Remediation  `json:"remediation"`
		} `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode envelope: %v (%q)", err, buf.String())
	}
	if got.Schema != "one-cli/error/v1" {
		t.Errorf("schema = %q, want one-cli/error/v1", got.Schema)
	}
	if got.Error.Code != "MY_CODE" || got.Error.Message != "boom" {
		t.Errorf("code/message mismatch: %+v", got.Error)
	}
	if got.Error.Context["k"] != "v" {
		t.Errorf("context dropped: %+v", got.Error.Context)
	}
	if len(got.Error.Remediation) != 1 || got.Error.Remediation[0].Action != "retry" {
		t.Errorf("remediation mismatch: %+v", got.Error.Remediation)
	}
}

func TestEmitError_NilContextRemediation_BecomeEmpty(t *testing.T) {
	// Important contract: agents pattern-match on shape, so nil maps must
	// surface as `{}` and nil slices as `[]`, never as `null`.
	env := NewError("X", "y").envelope()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"context":{}`) {
		t.Errorf("nil context should marshal as {}, got %q", s)
	}
	if !strings.Contains(s, `"remediation":[]`) {
		t.Errorf("nil remediation should marshal as [], got %q", s)
	}
}

func TestError_WithExit0(t *testing.T) {
	e := NewError("CANCELLED", "user exit").WithExit0()
	if !e.Exit0 {
		t.Errorf("Exit0 = false, want true")
	}
	// Original should not mutate (defensive copy).
	orig := NewError("CANCELLED", "user exit")
	if orig.Exit0 {
		t.Errorf("original Exit0 mutated")
	}
}

func TestRenderable_Implements_TTYRenderer(t *testing.T) {
	var r any = &renderable{Schema: "one-cli/test/v1", Name: "x"}
	tr, ok := r.(TTYRenderer)
	if !ok {
		t.Fatalf("renderable should implement TTYRenderer")
	}
	var buf bytes.Buffer
	tr.RenderTTY(&buf)
	if got := buf.String(); got != "hello x\n" {
		t.Errorf("RenderTTY output = %q, want %q", got, "hello x\n")
	}
}

func TestPlain_DoesNotImplement_TTYRenderer(t *testing.T) {
	var p any = &plain{Schema: "one-cli/test/v1", Value: 1}
	if _, ok := p.(TTYRenderer); ok {
		t.Errorf("plain should NOT implement TTYRenderer (would change Emit dispatch)")
	}
}
