package git

import "context"

type CreateBranchInput struct {
	Branch     string `json:"branch"`
	StartPoint string `json:"startPoint"`
	Checkout   bool   `json:"checkout"`
}

type BranchResult struct {
	Branch     string `json:"branch"`
	Head       string `json:"head"`
	CheckedOut bool   `json:"checkedOut"`
	Warning    string `json:"warning,omitempty"`
}

func (s *Service) CreateBranch(ctx context.Context, root string, input CreateBranchInput) (*BranchResult, error) {
	return s.backend.CreateBranch(ctx, root, input)
}
