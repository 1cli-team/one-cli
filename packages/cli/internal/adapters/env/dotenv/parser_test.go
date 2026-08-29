package dotenv

import (
	"reflect"
	"testing"
)

func TestParseDotenv_BasicShape(t *testing.T) {
	in := `# header comment
DATABASE_URL=postgres://localhost/db
JWT_SECRET="hunter2"
EMPTY=
QUOTED='single-quoted-value'

# blank lines + spacing
  PADDED  =  ok
`
	got := Parse(in)
	want := map[string]string{
		"DATABASE_URL": "postgres://localhost/db",
		"JWT_SECRET":   "hunter2",
		"EMPTY":        "",
		"QUOTED":       "single-quoted-value",
		"PADDED":       "ok",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse mismatch\n  got=%v\n  want=%v", got, want)
	}
}

func TestParseDotenv_IgnoresGarbage(t *testing.T) {
	in := `notakeyvalueline
=missing-key
  # comment
`
	got := Parse(in)
	if len(got) != 0 {
		t.Errorf("expected empty map; got %v", got)
	}
}

func TestSerializeDotenv_StableOrder(t *testing.T) {
	// Same input map, two iteration orders — output must be byte-identical.
	a := map[string]string{"B": "2", "A": "1", "C": "3"}
	b := map[string]string{"C": "3", "A": "1", "B": "2"}
	if Serialize(a) != Serialize(b) {
		t.Errorf("serialize order not deterministic")
	}
	want := "A=1\nB=2\nC=3\n"
	if got := Serialize(a); got != want {
		t.Errorf("serialize bytes mismatch\n  got=%q\n  want=%q", got, want)
	}
}

func TestSerializeDotenv_QuotesWhenNeeded(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"plain alphanum", "abc123", `abc123`},
		{"with space", "hello world", `"hello world"`},
		{"with hash", "tk_#abc", `"tk_#abc"`},
		{"with dollar", "$VAR", `"$VAR"`},
		{"with backslash", `a\b`, `"a\\b"`},
		{"with double-quote", `say "hi"`, `"say \"hi\""`},
		{"with newline", "line1\nline2", `"line1\nline2"`},
		{"empty", "", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := serializeValue(tc.value)
			if got != tc.want {
				t.Errorf("serializeValue(%q) = %q; want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestDotenvEqual(t *testing.T) {
	if !Equal(map[string]string{"A": "1"}, map[string]string{"A": "1"}) {
		t.Errorf("equal maps reported unequal")
	}
	if Equal(map[string]string{"A": "1"}, map[string]string{"A": "2"}) {
		t.Errorf("differing values reported equal")
	}
	if Equal(map[string]string{"A": "1"}, map[string]string{"A": "1", "B": "2"}) {
		t.Errorf("differing key counts reported equal")
	}
}
