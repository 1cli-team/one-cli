package creation

import (
	"encoding/json"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/jsonedit"
)

func marshalJSONValue(value any) ([]byte, error) { return jsonedit.Marshal(value) }
func updateJSONFields(raw []byte, updates map[string]json.RawMessage) ([]byte, error) {
	return jsonedit.UpdateFields(raw, updates)
}
