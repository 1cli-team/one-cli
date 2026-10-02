package workspace

import (
	"bytes"
	"strings"
	"testing"
)

func TestManifestGroupsRoundTripAndPreserveComments(t *testing.T) {
	raw := []byte("version = 2\n# keep workspace notes\n[projects.desktop-renderer]\npath='apps/desktop-renderer'\ntoolchain='node'\n[projects.desktop-main]\npath='services/desktop-main'\ntoolchain='node'\n[groups.desktop]\nprojects=['desktop-renderer','desktop-main']\n")
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Groups) != 1 || m.Groups[0].Name != "desktop" || len(m.Groups[0].Projects) != 2 {
		t.Fatalf("groups: %#v", m.Groups)
	}
	after, err := MarshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte("# keep workspace notes")) {
		t.Fatal("lost comments")
	}
	again, err := ParseManifest(after)
	if err != nil || len(again.Groups) != 1 {
		t.Fatalf("round trip: %s %v", after, err)
	}
}

func TestManifestRejectsInvalidGroups(t *testing.T) {
	project := "version=2\n[projects.web]\npath='apps/web'\ntoolchain='node'\n"
	for _, group := range []string{"[groups.web]\nprojects=['web']", "[groups.desktop]\nprojects=[]", "[groups.desktop]\nprojects=['missing']", "[groups.desktop]\nprojects=['web','web']", "[groups.'../desktop']\nprojects=['web']"} {
		if _, err := ParseManifest([]byte(project + group + "\n")); err == nil {
			t.Fatalf("accepted %s", group)
		}
	}
	m, err := ParseManifest([]byte(project + "[groups.desktop]\nprojects=['web']\n"))
	if err != nil {
		t.Fatal(err)
	}
	m.Projects = nil
	if _, err = MarshalManifest(m); err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("accepted orphan member: %v", err)
	}
}
