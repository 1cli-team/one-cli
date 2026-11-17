package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func TestErrorRenderingPreservesCauseAndProtocol(t *testing.T) {
	locale := i18n.Active()
	t.Cleanup(func() { SetMode(ModeAuto); _ = i18n.Init(locale) })
	err := NewError("TEST_FAILURE", "无法创建工作区 /tmp/示例\npermission denied").
		WithContext(map[string]any{"path": "/tmp/示例"}).
		WithRemediation(Remediation{Action: "retry", Hint: "选择其他目录", Command: "one create other-directory"})
	for _, mode := range []Mode{ModeTTY, ModeJSON, ModeYAML} {
		SetMode(mode)
		for _, language := range []string{"zh-CN", "en-US"} {
			if initErr := i18n.Init(language); initErr != nil {
				t.Fatal(initErr)
			}
			var buf bytes.Buffer
			emitErrorTo(&buf, err)
			if mode == ModeTTY {
				for _, want := range []string{"无法创建工作区 /tmp/示例", "permission denied", "选择其他目录", "one create other-directory", "TEST_FAILURE", i18n.T("error.try_label")} {
					if !strings.Contains(buf.String(), want) {
						t.Errorf("%s text missing %q: %s", language, want, buf.String())
					}
				}
				if strings.Contains(buf.String(), "\x1b") {
					t.Errorf("text redirected with ANSI escapes: %q", buf.String())
				}
			} else {
				var got errorEnvelope
				var decodeErr error
				if mode == ModeJSON {
					decodeErr = json.Unmarshal(buf.Bytes(), &got)
				} else {
					var fields map[string]any
					decodeErr = yaml.Unmarshal(buf.Bytes(), &fields)
					raw, _ := json.Marshal(fields)
					if decodeErr == nil {
						decodeErr = json.Unmarshal(raw, &got)
					}
				}
				if decodeErr != nil || !reflect.DeepEqual(got, err.envelope()) {
					t.Errorf("mode %d changed the error protocol: %s (%v)", mode, buf.String(), decodeErr)
				}
			}
		}
		var cancelled bytes.Buffer
		emitErrorTo(&cancelled, err.WithExit0())
		if cancelled.Len() != 0 {
			t.Errorf("mode %d printed a cancelled operation: %q", mode, cancelled.String())
		}
	}
}
