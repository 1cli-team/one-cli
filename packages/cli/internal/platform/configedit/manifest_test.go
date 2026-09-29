package configedit

import (
	"bytes"
	"testing"
)

func TestManifestTOMLEditsPreserveLayouts(t *testing.T) {
	for _, before := range []string{
		"# header\nversion = 2\nworkspace = { id = 'demo', name = 'Old' } # identity\nenv = { infisical = { projectId = 'p', environments = ['dev'] } }\nprojects = { web = { path = 'apps/web', toolchain = 'node' }, api = { path = 'services/api', toolchain = 'go' } }\n",
		"# header\nversion = 2\nworkspace.id = 'demo'\nworkspace.name = 'Old' # identity\nenv.infisical.projectId = 'p'\nenv.infisical.environments = ['dev']\nprojects.web.path = 'apps/web'\nprojects.web.toolchain = 'node'\nprojects.api.path = 'services/api'\nprojects.api.toolchain = 'go'\n",
		"# header\nversion = 2\n[workspace]\nid = 'demo'\nname = 'Old' # identity\n[env.infisical]\nprojectId = 'p'\nenvironments = [\n # chosen environment\n 'dev',\n]\n[projects.web]\npath = 'apps/web'\ntoolchain = 'node'\n[projects.api]\npath = 'services/api'\ntoolchain = 'go'\n",
	} {
		t.Run(before[:min(35, len(before))], func(t *testing.T) {
			desired := []byte("version = 2\n[workspace]\nid = 'demo'\nname = 'New'\n[projects.web]\npath = 'apps/web'\ntoolchain = 'node'\ntemplate = 'react-spa'\n")
			got, err := UpdateTOML([]byte(before), desired)
			if err != nil {
				t.Fatal(err)
			}
			for _, keep := range []string{"# header", "# identity", "'apps/web'", "'node'"} {
				if !bytes.Contains(got, []byte(keep)) {
					t.Fatalf("lost %s: %s", keep, got)
				}
			}
			if bytes.Contains(got, []byte(metadataPrefix)) {
				t.Fatalf("added ownership comment: %s", got)
			}
			again, err := UpdateTOML(got, desired)
			if err != nil || !bytes.Equal(got, again) {
				t.Fatalf("update not idempotent: %s %v", again, err)
			}
		})
	}
}
