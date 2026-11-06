package creation

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"gopkg.in/yaml.v3"
)

func planFlutterWorkspace(files *fsutil.FilePlan, dirs []string) error {
	raw, err := files.Read("pubspec.yaml")
	if err != nil {
		return err
	}
	if raw == nil {
		raw = []byte("name: _\npublish_to: none\nenvironment:\n  sdk: ^3.6.0\n")
	}
	root, err := workspace.ParseFlutterProject(raw)
	if err != nil {
		return err
	}
	members, err := includeProjects(root.Workspace, dirs)
	if err != nil {
		return err
	}
	names := map[string]string{}
	if root.Name != "" {
		names[root.Name] = "pubspec.yaml"
	}
	for _, dir := range dirs {
		file := filepath.ToSlash(filepath.Join(dir, "pubspec.yaml"))
		data, err := files.Read(file)
		if err != nil {
			return err
		}
		project, err := workspace.ParseFlutterProject(data)
		if err != nil {
			return err
		}
		if project.Name == "" {
			return i18n.Errorf("flutter.name_missing", file)
		}
		if previous, exists := names[project.Name]; exists {
			return i18n.Errorf("build.duplicate_package", project.Name, previous, file)
		}
		names[project.Name] = file
		if project.Resolution != "" && project.Resolution != "workspace" {
			return i18n.Errorf("flutter.resolution_invalid", file)
		}
		after, err := updatePubspec(data, map[string]any{"resolution": "workspace"})
		if err != nil {
			return err
		}
		if err := files.Set(file, after, 0o644); err != nil {
			return err
		}
	}
	updates := map[string]any{"workspace": members}
	if root.Environment["sdk"] == "" {
		environment := root.Environment
		if environment == nil {
			environment = map[string]string{}
		}
		environment["sdk"] = "^3.6.0"
		updates["environment"] = environment
	}
	after, err := updatePubspec(raw, updates)
	if err != nil {
		return err
	}
	if err := files.Set("pubspec.yaml", after, 0o644); err != nil {
		return err
	}
	ignore, err := files.Read(".gitignore")
	if err != nil {
		return err
	}
	text := strings.TrimRight(string(ignore), "\r\n")
	for _, pattern := range []string{".dart_tool/", ".flutter-plugins", ".flutter-plugins-dependencies", ".pub-cache/", "build/"} {
		if !strings.Contains("\n"+text+"\n", "\n"+pattern+"\n") {
			text += "\n" + pattern
		}
	}
	return files.Set(".gitignore", []byte(text+"\n"), 0o644)
}

// Edit the few fields One owns while retaining native pub fields and comments.
func updatePubspec(raw []byte, updates map[string]any) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, i18n.Errorf("flutter.pubspec_mapping")
	}
	root := doc.Content[0]
	// Keep insertion order deterministic without reordering existing fields.
	for _, key := range []string{"environment", "resolution", "workspace"} {
		value, ok := updates[key]
		if !ok {
			continue
		}
		var node yaml.Node
		if err := node.Encode(value); err != nil {
			return nil, err
		}
		found := false
		for i := 0; i < len(root.Content); i += 2 {
			if root.Content[i].Value == key {
				previous := root.Content[i+1]
				if key == "workspace" && previous.Kind == yaml.SequenceNode {
					// includeProjects retains the original prefix. Append members
					// without discarding comments attached to existing entries.
					previous.Content = append(previous.Content, node.Content[len(previous.Content):]...)
					found = true
					break
				}
				if key == "environment" && previous.Kind == yaml.MappingNode {
					// The only generated environment change is a missing SDK
					// constraint. Other constraints and their comments stay intact.
					var sdk *yaml.Node
					for j := 0; j < len(previous.Content); j += 2 {
						if previous.Content[j].Value == "sdk" {
							sdk = previous.Content[j+1]
						}
					}
					if sdk == nil {
						sdk = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str"}
						previous.Content = append(previous.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "sdk"}, sdk)
					}
					sdk.Tag, sdk.Value = "!!str", "^3.6.0"
					found = true
					break
				}
				node.HeadComment, node.LineComment, node.FootComment = previous.HeadComment, previous.LineComment, previous.FootComment
				root.Content[i+1] = &node
				found = true
				break
			}
		}
		if !found {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &node)
		}
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
