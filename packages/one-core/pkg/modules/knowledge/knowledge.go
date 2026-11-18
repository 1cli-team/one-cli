// Package knowledge defines the workspace knowledge service used by One's interfaces.
package knowledge

import (
	"context"
	"io"
)

type Backend interface {
	Connection(ctx context.Context) (*Connection, error)
	Configure(ctx context.Context, input ConnectionInput) (Connection, error)
	TestConnection(ctx context.Context, input ConnectionInput) (Connection, error)
	Check(ctx context.Context, workspaceID string) (Binding, error)
	Browse(ctx context.Context, uri string) (Directories, error)
	Import(ctx context.Context, workspaceID, name string, source io.Reader) (Import, error)
	ImportStatus(ctx context.Context, workspaceID, taskID string) (Import, error)
	Status(ctx context.Context, workspaceID string) (Status, error)
	Bind(ctx context.Context, workspaceID, uri string) (Binding, error)
	Unbind(ctx context.Context, workspaceID string) error
	Find(ctx context.Context, workspaceID, query string) ([]Match, error)
	Read(ctx context.Context, workspaceID, uri string) (Content, error)
	WorkspaceAgent(ctx context.Context, workspace Workspace) (AgentSettings, error)
	SetWorkspaceAgent(ctx context.Context, workspace Workspace, input AgentSettingsUpdate) (AgentSettings, error)
	Sessions(ctx context.Context, workspace Workspace) ([]Session, error)
	Session(ctx context.Context, workspace Workspace, id, archive string) (Session, error)
	Remember(ctx context.Context, workspace Workspace, text string) (Session, error)
	Memories(ctx context.Context, workspace Workspace, query string) ([]Match, error)
	Memory(ctx context.Context, workspace Workspace, uri string) (Content, error)
	Skills(ctx context.Context, workspace Workspace, query string) ([]Skill, error)
	Skill(ctx context.Context, workspace Workspace, ref SkillRef) (Skill, error)
	SaveSkill(ctx context.Context, workspace Workspace, input SkillInput) (string, error)
	DeleteSkill(ctx context.Context, workspace Workspace, ref SkillRef) error
	SkillForAgent(ctx context.Context, workspace Workspace, ref SkillRef, path string) (string, error)
	Recall(ctx context.Context, workspace Workspace, query string) (AgentContext, error)
	RecordTurn(ctx context.Context, workspace Workspace, turn Turn) (Session, error)
}

type Connection struct {
	URL       string `json:"url"`
	HasAPIKey bool   `json:"hasApiKey"`
	Account   string `json:"account,omitempty"`
	User      string `json:"user,omitempty"`
}

type ConnectionInput struct {
	URL    string  `json:"url"`
	APIKey *string `json:"apiKey,omitempty"`
}

type Binding struct {
	URI string `json:"uri"`
}

type Match struct {
	URI      string  `json:"uri"`
	Abstract string  `json:"abstract"`
	Score    float64 `json:"score"`
}

type Content struct {
	URI         string `json:"uri"`
	Text        string `json:"text"`
	IsDirectory bool   `json:"isDirectory"`
}

type Status struct {
	Configured  bool     `json:"configured"`
	NeedsRebind bool     `json:"needsRebind"`
	Binding     *Binding `json:"binding"`
}

// MaxImportSize bounds the selected file before sending it to OpenViking.
const MaxImportSize = 20 << 20

type Import struct {
	TaskID string `json:"taskId"`
	URI    string `json:"uri,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Directory struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type Directories struct {
	Current string      `json:"current"`
	Parent  string      `json:"parent,omitempty"`
	Entries []Directory `json:"entries"`
}
