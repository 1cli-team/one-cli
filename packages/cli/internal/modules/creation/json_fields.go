package creation

import (
	"encoding/json"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/jsonedit"
)

func marshalJSONValue(value any) ([]byte, error) { return jsonedit.Marshal(value) }
func updateJSONFields(raw []byte, updates map[string]json.RawMessage) ([]byte, error) {
	return jsonedit.UpdateFields(raw, updates)
}
