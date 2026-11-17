// Package issuetracking defines the project-scoped issue capability shared by CLI and Desktop.
package issuetracking

import (
	"context"
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Error struct {
	Code        string `json:"code"`
	OperationID string `json:"operationId,omitempty"`
}

func (e *Error) Error() string { return i18n.T("linear.error." + e.Code) }
func Fail(code string) error   { return &Error{Code: code} }

type Scope struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type OAuthConfig struct {
	ClientID    string `json:"clientId"`
	RedirectURI string `json:"redirectUri"`
}

type Connection struct {
	ID                string `json:"id"`
	UserID            string `json:"userId"`
	Name              string `json:"name"`
	OrganizationID    string `json:"organizationId"`
	OrganizationName  string `json:"organizationName"`
	CanCreateProjects bool   `json:"canCreateProjects"`
}

type Authorization struct {
	ID        string `json:"id"`
	URL       string `json:"url,omitempty"`
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
}

type ConnectionState struct {
	Config        OAuthConfig    `json:"config"`
	Connections   []Connection   `json:"connections"`
	Authorization *Authorization `json:"authorization,omitempty"`
}

type Named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type State struct {
	Named
	Type     string  `json:"type"`
	Position float64 `json:"position"`
}

type Team struct {
	Named
	Key     string  `json:"key"`
	States  []State `json:"states"`
	Members []Named `json:"members"`
	Labels  []Named `json:"labels"`
}

type Project struct {
	Named
	Description string  `json:"description"`
	URL         string  `json:"url"`
	Teams       []Named `json:"teams"`
}

type Binding struct {
	Scope          Scope   `json:"scope"`
	ConnectionID   string  `json:"connectionId"`
	OrganizationID string  `json:"organizationId"`
	Project        Project `json:"project"`
	TeamID         string  `json:"teamId"`
}

type PageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type Projects struct {
	Nodes    []Project `json:"nodes"`
	PageInfo PageInfo  `json:"pageInfo"`
}

type ProjectInput struct {
	ConnectionID string `json:"connectionId"`
	Name         string `json:"name"`
	TeamID       string `json:"teamId"`
}

type Comment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	User      *Named `json:"user"`
}

type Issue struct {
	ID               string    `json:"id"`
	Identifier       string    `json:"identifier"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	URL              string    `json:"url"`
	UpdatedAt        string    `json:"updatedAt"`
	Priority         int       `json:"priority"`
	Team             Named     `json:"team"`
	Project          *Named    `json:"project"`
	State            State     `json:"state"`
	Assignee         *Named    `json:"assignee"`
	Labels           []Named   `json:"labels"`
	Comments         []Comment `json:"comments"`
	CommentsPageInfo PageInfo  `json:"commentsPageInfo"`
}

type IssueFilter struct {
	After      string `json:"after"`
	Query      string `json:"query"`
	TeamID     string `json:"teamId"`
	StateID    string `json:"stateId"`
	AssigneeID string `json:"assigneeId"`
	Priority   *int   `json:"priority,omitempty"`
}

type Issues struct {
	Nodes     []Issue  `json:"nodes"`
	PageInfo  PageInfo `json:"pageInfo"`
	FetchedAt string   `json:"fetchedAt"`
	Stale     bool     `json:"stale"`
}

type IssueInput struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	TeamID            string   `json:"teamId"`
	StateID           string   `json:"stateId"`
	AssigneeID        string   `json:"assigneeId"`
	Priority          int      `json:"priority"`
	LabelIDs          []string `json:"labelIds"`
	ExpectedUpdatedAt string   `json:"expectedUpdatedAt,omitempty"`
	OperationID       string   `json:"operationId,omitempty"`
}

type Operation struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	ResourceID string `json:"resourceId"`
}

// Constructors must be inert. Credentials are resolved only while executing a request.
type Backend interface {
	Status(context.Context) (ConnectionState, error)
	Configure(context.Context, OAuthConfig) error
	Authorize(context.Context) (Authorization, error)
	AuthorizeProjectCreation(context.Context) (Authorization, error)
	CancelAuthorization()
	Disconnect(context.Context, string) error
	Projects(context.Context, string, string) (Projects, error)
	Project(context.Context, string, string) (Project, error)
	CreateProject(context.Context, string, ProjectInput) (Project, error)
	Teams(context.Context, string) ([]Named, error)
	Team(context.Context, string, string) (Team, error)
	Issues(context.Context, Binding, IssueFilter) (Issues, error)
	Issue(context.Context, Binding, string) (Issue, error)
	CreateIssue(context.Context, Binding, IssueInput) (Issue, error)
	UpdateIssue(context.Context, Binding, string, IssueInput) (Issue, error)
	Comment(context.Context, Binding, string, string, string) (Comment, error)
	Comments(context.Context, Binding, string, string) ([]Comment, PageInfo, error)
	Operation(context.Context, Binding, string) (Operation, error)
	PendingOperations(context.Context, Binding) ([]Operation, error)
}

func (s Scope) Key() string { return fmt.Sprintf("%s:%s", s.Kind, s.ID) }
