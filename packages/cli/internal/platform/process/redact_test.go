package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRedactedChild(t *testing.T) {
	if os.Getenv("ONE_REDACTION_CHILD") != "1" {
		return
	}
	value := os.Getenv("ONE_REDACTION_SECRET")
	// Separate writes exercise the same boundary used by application logs.
	for _, b := range []byte(value) {
		fmt.Fprintf(os.Stdout, "%c", b)
		fmt.Fprintf(os.Stderr, "%c", b)
	}
	fmt.Fprint(os.Stdout, "\nnormal stdout\n")
	fmt.Fprint(os.Stderr, "\nnormal stderr\n")
	os.Exit(7)
}
func TestRunRedactedMasksBothStreamsAndKeepsExitCode(t *testing.T) {
	const value = "private-runtime-test-value"
	child := Command(os.Args[0], "-test.run=^TestRedactedChild$")
	child.Env = append(os.Environ(), "ONE_REDACTION_CHILD=1", "ONE_REDACTION_SECRET="+value)
	var out, errOut bytes.Buffer
	child.Stdout, child.Stderr = &out, &errOut
	err := RunRedacted(context.Background(), child, map[string]string{"ONE_REDACTION_SECRET": value})
	if ExitCode(err) != 7 {
		t.Fatalf("exit status changed: %v", err)
	}
	for _, log := range []string{out.String(), errOut.String()} {
		if strings.Contains(log, value) || !strings.Contains(log, "[REDACTED]") || !strings.Contains(log, "normal") {
			t.Fatalf("unsafe or missing output: %q", log)
		}
	}
}
