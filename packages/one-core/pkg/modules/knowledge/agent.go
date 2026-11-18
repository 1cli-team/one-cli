package knowledge

// Workspace is resolved by One's registry before accessing the project's context.
// ID is from the manifest and is stable across worktrees and machines.
type Workspace struct {
	EntryID string
	ID      string
}

type AgentSettings struct {
	Generation string     `json:"generation,omitempty"`
	Automatic  bool       `json:"automatic"`
	Skills     []SkillRef `json:"skills"`
}

type AgentSettingsUpdate struct {
	Automatic *bool     `json:"automatic,omitempty"`
	Skill     *SkillRef `json:"skill,omitempty"`
	Enabled   bool      `json:"enabled,omitempty"`
}

type SkillRef struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

type Skill struct {
	SkillRef
	Description string   `json:"description"`
	URI         string   `json:"uri"`
	Content     string   `json:"content,omitempty"`
	Attachments bool     `json:"attachments,omitempty"`
	Files       []string `json:"files,omitempty"`
}

type SkillInput struct {
	SkillRef
	Source  string `json:"source"`
	Replace bool   `json:"replace,omitempty"`
}

type Session struct {
	ID          string    `json:"id"`
	Title       string    `json:"title,omitempty"`
	ThreadID    string    `json:"threadId,omitempty"`
	Archives    int       `json:"archives"`
	Archive     string    `json:"archive,omitempty"`
	Messages    []Message `json:"messages,omitempty"`
	TaskID      string    `json:"taskId,omitempty"`
	Status      string    `json:"status,omitempty"`
	MemoryCount *int      `json:"memoryCount,omitempty"`
}

type SessionLink struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	TaskID      string `json:"taskId,omitempty"`
	Status      string `json:"status,omitempty"`
}

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type Turn struct {
	Generation string
	ThreadID   string
	TurnID     string
	Title      string
	Messages   []Message
}

type AgentContext struct {
	Memories []Content
	Skills   []SkillRef
}
