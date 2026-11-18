package serve

import (
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func readKnowledgeInput(w http.ResponseWriter, r *http.Request, input any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := decodeJSON(r, input); err != nil {
		return cliErrors.New(cliErrors.SERVE_PAYLOAD_INVALID, i18n.T("openviking.invalid_body"))
	}
	return nil
}

func registerKnowledgeAgentRoutes(handle func(string, knowledgeOperation), backend knowledge.Backend, base string) {
	handle("GET "+base+"/agent", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.WorkspaceAgent(r.Context(), workspace)
	})
	handle("PUT "+base+"/agent", func(w http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		var input knowledge.AgentSettingsUpdate
		if err := readKnowledgeInput(w, r, &input, 16<<10); err != nil {
			return nil, err
		}
		return backend.SetWorkspaceAgent(r.Context(), workspace, input)
	})
	handle("GET "+base+"/sessions", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Sessions(r.Context(), workspace)
	})
	handle("GET "+base+"/sessions/{sessionId}", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Session(r.Context(), workspace, r.PathValue("sessionId"), r.URL.Query().Get("archive"))
	})
	handle("GET "+base+"/memories", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Memories(r.Context(), workspace, r.URL.Query().Get("query"))
	})
	handle("GET "+base+"/memory", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Memory(r.Context(), workspace, r.URL.Query().Get("uri"))
	})
	handle("POST "+base+"/memories", func(w http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		var input struct {
			Text string `json:"text"`
		}
		if err := readKnowledgeInput(w, r, &input, 192<<10); err != nil {
			return nil, err
		}
		return backend.Remember(r.Context(), workspace, input.Text)
	})
	handle("GET "+base+"/skills", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Skills(r.Context(), workspace, r.URL.Query().Get("query"))
	})
	ref := func(r *http.Request) knowledge.SkillRef {
		return knowledge.SkillRef{Name: r.PathValue("skillName"), Scope: r.URL.Query().Get("scope")}
	}
	handle("GET "+base+"/skills/{skillName}", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Skill(r.Context(), workspace, ref(r))
	})
	handle("POST "+base+"/skills", func(w http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		var input knowledge.SkillInput
		if err := readKnowledgeInput(w, r, &input, 3<<20); err != nil {
			return nil, err
		}
		taskID, err := backend.SaveSkill(r.Context(), workspace, input)
		return map[string]string{"taskId": taskID}, err
	})
	handle("DELETE "+base+"/skills/{skillName}", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return struct{}{}, backend.DeleteSkill(r.Context(), workspace, ref(r))
	})
}
