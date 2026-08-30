package infisical

import (
	"reflect"
	"testing"
)

// folderStepsFor is the only piece of EnsureFolder we can unit-test
// without standing up an Infisical mock — the SDK call itself is a
// one-liner. These cases lock the path-splitting contract: each step
// must name exactly one folder and its parent (which itself was just
// created or already exists).
func TestFolderStepsFor(t *testing.T) {
	cases := []struct {
		in   string
		want []folderStep
	}{
		{in: "/", want: nil},
		{in: "", want: nil},
		{in: ".", want: nil},
		{in: "/services", want: []folderStep{{Name: "services", Parent: "/"}}},
		{in: "services", want: []folderStep{{Name: "services", Parent: "/"}}},
		{in: "/services/api", want: []folderStep{
			{Name: "services", Parent: "/"},
			{Name: "api", Parent: "/services"},
		}},
		{in: "/a/b/c", want: []folderStep{
			{Name: "a", Parent: "/"},
			{Name: "b", Parent: "/a"},
			{Name: "c", Parent: "/a/b"},
		}},
		// NormalizePath collapses repeats and trims, so this should match
		// the simple form.
		{in: "//services///api/", want: []folderStep{
			{Name: "services", Parent: "/"},
			{Name: "api", Parent: "/services"},
		}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := folderStepsFor(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("folderStepsFor(%q):\n  want: %#v\n  got:  %#v", c.in, c.want, got)
			}
		})
	}
}

// TestParseFolderNotFound locks the regex against the real Infisical
// 404 wire format the user surfaced. Without a regex, the raw multi-line
// SDK dump leaks into the error envelope and is unreadable.
func TestParseFolderNotFound(t *testing.T) {
	cases := []struct {
		msg        string
		wantFolder string
		wantEnv    string
	}{
		{
			msg:        `APIError: CallListSecretsV3Raw unsuccessful response [GET https://app.infisical.com/api/v3/secrets/raw?...] [status-code=404] [reqId=req-X] [message="Folder with path '/apps' in environment 'prod' was not found. Please ensure the environment slug and secret path is correct."]`,
			wantFolder: "/apps",
			wantEnv:    "prod",
		},
		{
			msg:        `Folder with path '/services/api/v2' in environment 'staging' was not found.`,
			wantFolder: "/services/api/v2",
			wantEnv:    "staging",
		},
		{msg: `404 Not Found`, wantFolder: "", wantEnv: ""},
		{msg: ``, wantFolder: "", wantEnv: ""},
	}
	for _, c := range cases {
		t.Run(c.msg[:min(len(c.msg), 40)], func(t *testing.T) {
			folder, env := parseFolderNotFound(errFromMsg(c.msg))
			if folder != c.wantFolder || env != c.wantEnv {
				t.Errorf("parseFolderNotFound: want (%q,%q), got (%q,%q)", c.wantFolder, c.wantEnv, folder, env)
			}
		})
	}
	folder, env := parseFolderNotFound(nil)
	if folder != "" || env != "" {
		t.Errorf("parseFolderNotFound(nil): want empty, got (%q,%q)", folder, env)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestIsAlreadyExistsError(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{msg: "folder already exists", want: true},
		{msg: "Already Exists in environment", want: true},
		{msg: "duplicate key", want: true},
		{msg: "409 conflict on /services", want: true},
		{msg: "Conflict: name taken", want: true},
		{msg: "not found", want: false},
		{msg: "404 path not found", want: false},
		{msg: "", want: false},
	}
	for _, c := range cases {
		t.Run(c.msg, func(t *testing.T) {
			got := isAlreadyExistsError(errFromMsg(c.msg))
			if got != c.want {
				t.Errorf("isAlreadyExistsError(%q): want %v, got %v", c.msg, c.want, got)
			}
		})
	}
	if isAlreadyExistsError(nil) {
		t.Errorf("isAlreadyExistsError(nil) must be false")
	}
}

// errFromMsg returns a tiny error wrapping msg without depending on the
// errors package's Wrap / fmt.Errorf — keeps the test focused on
// substring matching, not error chain traversal.
type stringErr string

func (e stringErr) Error() string { return string(e) }

func errFromMsg(msg string) error {
	if msg == "" {
		return nil
	}
	return stringErr(msg)
}
