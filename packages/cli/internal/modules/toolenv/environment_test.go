package toolenv

import (
	"reflect"
	"strings"
	"testing"
)

func TestEnvironmentAssignmentsRemovalsAndLiteralValues(t *testing.T) {
	data := "Remove-Item -ErrorAction SilentlyContinue -LiteralPath 'Env:/GONE'\n${Env:EMPTY}=''\n${Env:VALUE}='中文 ''quoted'' $HOME `literal`\nline\r\n'\n${Env:PATH}='C:\\Tools;C:\\Program Files\\bin'\n"
	result, err := parse(data, []string{"GONE=old", "KEEP=ok", "VALUE=before"})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, entry := range result {
		k, v, _ := strings.Cut(entry, "=")
		values[k] = v
	}
	want := map[string]string{"KEEP": "ok", "EMPTY": "", "VALUE": "中文 'quoted' $HOME `literal`\nline\r\n", "PATH": `C:\Tools;C:\Program Files\bin`}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("got %#v", values)
	}
}
func TestEnvironmentParserRejectsCodeWithoutEchoingData(t *testing.T) {
	for _, raw := range []string{"${Env:VALUE}='private-value';Invoke-Expression 'bad'\n", "unexpected private-value", "${Env:KEY}='private-value"} {
		_, err := parse(raw, nil)
		if err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatal(err)
		}
	}
}
