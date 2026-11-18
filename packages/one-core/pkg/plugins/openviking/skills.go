package openviking

import (
	"context"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

const userSkills = "viking://~/skills"
const sharedSkills = "viking://agent/skills"

var skillName = regexp.MustCompile("^[A-Za-z0-9_-]{1,64}$")

func validSkillRef(ref knowledge.SkillRef) bool {
	return skillName.MatchString(ref.Name) && (ref.Scope == "user" || ref.Scope == "shared")
}

func skillRoot(scope string) string {
	if scope == "shared" {
		return sharedSkills
	}
	return userSkills
}

type skillResult struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Root        string                  `json:"root_uri"`
	URI         string                  `json:"skill_md_uri"`
	Content     string                  `json:"content"`
	Files       []struct{ Path string } `json:"files"`
}

func (s skillResult) public() (knowledge.Skill, bool) {
	scope := "user"
	switch {
	case strings.HasPrefix(s.Root, sharedSkills+"/"):
		scope = "shared"
	case strings.HasPrefix(s.Root, userSkills+"/"):
	case strings.HasPrefix(s.Root, "viking://user/") && strings.HasSuffix(s.Root, "/skills/"+s.Name):
	default:
		return knowledge.Skill{}, false
	}
	ref := knowledge.SkillRef{Name: s.Name, Scope: scope}
	attachments := false
	for _, file := range s.Files {
		if file.Path != "" && file.Path != "SKILL.md" && !strings.HasPrefix(file.Path, ".") {
			attachments = true
			break
		}
	}
	return knowledge.Skill{SkillRef: ref, Description: s.Description, URI: s.URI, Content: s.Content, Attachments: attachments, Files: skillFiles(s)}, validSkillRef(ref)
}

func skillFiles(skill skillResult) []string {
	var result []string
	for _, file := range skill.Files {
		if file.Path == "" || file.Path == "SKILL.md" || !safeSkillPath(file.Path) {
			continue
		}
		result = append(result, file.Path)
	}
	return result
}

func safeSkillPath(value string) bool {
	if value == "" || strings.ContainsAny(value, "\\%?#") || path.IsAbs(value) || path.Clean(value) != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

func (p *Plugin) Skills(ctx context.Context, workspace knowledge.Workspace, query string) ([]knowledge.Skill, error) {
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return nil, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return nil, err
	}
	if len(query) > 8192 {
		return nil, agentInput("openviking.invalid_query")
	}
	targets := []string{userSkills, sharedSkills}
	var response map[string]any
	if strings.TrimSpace(query) == "" {
		response, err = client.ListSkills(ctx, nil)
	} else {
		response, err = client.FindSkills(ctx, query, &ov.FindSkillsOptions{Limit: 30, TargetURI: targets})
	}
	if err != nil {
		return nil, sdkError(err)
	}
	found, err := decodeResult[[]skillResult](response["skills"])
	if err != nil {
		return nil, err
	}
	result := make([]knowledge.Skill, 0, len(found))
	for _, item := range found {
		if info, ok := item.public(); ok {
			result = append(result, info)
		}
	}
	return result, nil
}

func getSkill(ctx context.Context, client *ov.Client, ref knowledge.SkillRef) (knowledge.Skill, error) {
	if !validSkillRef(ref) {
		return knowledge.Skill{}, agentInput("openviking.invalid_skill")
	}
	response, err := client.GetSkill(ctx, ref.Name, &ov.GetSkillOptions{
		TargetURI: skillRoot(ref.Scope), IncludeContent: new(true), IncludeFiles: new(true),
	})
	if err != nil {
		return knowledge.Skill{}, sdkError(err)
	}
	item, err := decodeResult[skillResult](response)
	if err != nil {
		return knowledge.Skill{}, err
	}
	info, ok := item.public()
	if !ok || info.SkillRef != ref {
		return knowledge.Skill{}, invalidResponse()
	}
	return info, nil
}

func (p *Plugin) Skill(ctx context.Context, workspace knowledge.Workspace, ref knowledge.SkillRef) (knowledge.Skill, error) {
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return knowledge.Skill{}, err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return knowledge.Skill{}, err
	}
	return getSkill(ctx, client, ref)
}

func (p *Plugin) SkillForAgent(ctx context.Context, workspace knowledge.Workspace, ref knowledge.SkillRef, file string) (string, error) {
	saved, binding, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return "", err
	}
	if !slices.Contains(binding.Agent.Skills, ref) {
		return "", outsideScope()
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return "", err
	}
	skill, err := getSkill(ctx, client, ref)
	if err != nil {
		return "", err
	}
	if file == "" || file == "SKILL.md" {
		text := skill.Content
		if len(skill.Files) > 0 {
			text += "\n\nFiles available to read:\n" + strings.Join(skill.Files, "\n")
		}
		return text, nil
	}
	if !slices.Contains(skill.Files, file) {
		return "", outsideScope()
	}
	key := (&url.URL{Path: file}).EscapedPath()
	text, err := client.Read(ctx, skillRoot(ref.Scope)+"/"+ref.Name+"/"+key, 0, -1)
	return text, sdkError(err)
}

func skillData(source string) (any, bool, error) {
	value := strings.TrimSpace(source)
	if len(value) > 512<<10 {
		return nil, false, agentInput("openviking.invalid_skill_source")
	}
	if strings.HasPrefix(value, "---\n") || strings.HasPrefix(value, "---\r\n") {
		// A structured payload prevents the SDK from interpreting user content
		// as a local server path. ValidateSkill reads and normalizes frontmatter.
		return value, true, nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || len(strings.Split(strings.Trim(u.Path, "/"), "/")) < 2 {
		return nil, false, agentInput("openviking.invalid_skill_source")
	}
	return u.String(), false, nil
}

func (p *Plugin) SaveSkill(ctx context.Context, workspace knowledge.Workspace, input knowledge.SkillInput) (string, error) {
	if input.Scope != "user" && input.Scope != "shared" || input.Replace && !validSkillRef(input.SkillRef) {
		return "", agentInput("openviking.invalid_skill")
	}
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return "", err
	}
	data, inline, err := skillData(input.Source)
	if err != nil {
		return "", err
	}
	client, err := sdk(saved, 45*time.Second)
	if err != nil {
		return "", err
	}
	target := skillRoot(input.Scope)
	if inline {
		validated, err := client.ValidateSkill(ctx, data, &ov.ValidateSkillOptions{TargetURI: target})
		if err != nil {
			return "", sdkError(err)
		}
		valid, _ := validated["valid"].(bool)
		name, _ := validated["name"].(string)
		description, _ := validated["description"].(string)
		if !valid || !skillName.MatchString(name) || input.Replace && name != input.Name {
			return "", agentInput("openviking.invalid_skill_source")
		}
		data = map[string]any{"name": name, "description": description, "content": data}
		if input.Replace {
			existing, err := getSkill(ctx, client, input.SkillRef)
			if err != nil {
				return "", err
			}
			if existing.Attachments {
				return "", agentInput("openviking.skill_use_package")
			}
		}
	}
	var result map[string]any
	if input.Replace {
		result, err = client.UpdateSkill(ctx, input.Name, data, &ov.UpdateSkillOptions{TargetURI: target})
	} else {
		result, err = client.AddSkill(ctx, data, &ov.AddSkillOptions{TargetURI: target})
	}
	if err != nil {
		return "", sdkError(err)
	}
	task, _ := result["task_id"].(string)
	return task, nil
}

func (p *Plugin) DeleteSkill(ctx context.Context, workspace knowledge.Workspace, ref knowledge.SkillRef) error {
	if !validSkillRef(ref) {
		return agentInput("openviking.invalid_skill")
	}
	saved, _, err := p.agentConnection(ctx, workspace)
	if err != nil {
		return err
	}
	client, err := sdk(saved, 0)
	if err != nil {
		return err
	}
	_, err = client.DeleteSkill(ctx, ref.Name, &ov.DeleteSkillOptions{TargetURI: skillRoot(ref.Scope)})
	return sdkError(err)
}
