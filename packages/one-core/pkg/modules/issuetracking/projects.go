package issuetracking

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type projectCreation struct {
	ProjectInput
	ID string `json:"id"`
}

func (s *Service) CreateAndBind(ctx context.Context, scope Scope, input ProjectInput) (Binding, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.TeamID == "" {
		return Binding{}, Fail("project_input")
	}
	resolved, err := s.Resolve(ctx, scope)
	if err != nil {
		return Binding{}, err
	}
	connection, err := s.connection(ctx, input.ConnectionID)
	if err != nil {
		return Binding{}, err
	}
	if !connection.CanCreateProjects {
		return Binding{}, Fail("project_write_scope")
	}
	lock, err := lockProjectCreation(resolved)
	if err != nil {
		return Binding{}, err
	}
	defer lock.Unlock()

	creation, fresh, err := s.store.reserveCreation(ctx, resolved, input)
	if err != nil {
		return Binding{}, err
	}

	var project Project
	if !fresh {
		project, err = s.Backend.Project(ctx, connection.ID, creation.ID)
		if err != nil && !hasCode(err, "not_found") {
			return Binding{}, err
		}
	}
	if project.ID == "" {
		project, err = s.Backend.CreateProject(ctx, creation.ID, input)
		if err != nil {
			if fresh && hasCode(err, "forbidden", "reconnect", "not_found", "rate_limit", "connection") {
				if clearErr := s.store.finishCreation(ctx, resolved, creation, nil); clearErr != nil {
					return Binding{}, clearErr
				}
				return Binding{}, err
			}
			project, err = s.Backend.Project(ctx, connection.ID, creation.ID)
			if err != nil || project.ID != creation.ID {
				return Binding{}, &Error{Code: "project_create_unknown", OperationID: creation.ID}
			}
		}
	}
	if project.ID != creation.ID {
		return Binding{}, &Error{Code: "project_create_unknown", OperationID: creation.ID}
	}
	binding, err := newBinding(resolved, connection, project, input.TeamID)
	if err != nil {
		return Binding{}, err
	}
	return binding, s.store.finishCreation(ctx, resolved, creation, &binding)
}

// Keep the creation lock separate so other bindings can be read and updated while Linear responds.
func lockProjectCreation(scope Scope) (*flock.Flock, error) {
	root, err := userdirs.Config()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte(scope.Key()))
	lock := flock.New(filepath.Join(root, fmt.Sprintf("linear-project-%x.lock", key)))
	locked, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, Fail("busy")
	}
	return lock, nil
}

func hasCode(err error, codes ...string) bool {
	var problem *Error
	return errors.As(err, &problem) && slices.Contains(codes, problem.Code)
}

func (s *bindingStore) reserveCreation(ctx context.Context, scope Scope, input ProjectInput) (projectCreation, bool, error) {
	var creation projectCreation
	var fresh bool
	err := s.access(ctx, func(doc *bindingDocument) (bool, error) {
		key := scope.Key()
		if _, ok := doc.Bindings[key]; ok {
			return false, Fail("project_bound")
		}
		if previous, ok := doc.Creations[key]; ok {
			if previous.ProjectInput != input {
				return false, Fail("project_create_pending")
			}
			creation = previous
			return false, nil
		}
		if doc.Creations == nil {
			doc.Creations = map[string]projectCreation{}
		}
		var id [16]byte
		_, _ = rand.Read(id[:])
		id[6] = (id[6] & 0x0f) | 0x40
		id[8] = (id[8] & 0x3f) | 0x80
		creation = projectCreation{ProjectInput: input, ID: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])}
		doc.Creations[key] = creation
		fresh = true
		return true, nil
	})
	return creation, fresh, err
}

func (s *bindingStore) finishCreation(ctx context.Context, scope Scope, creation projectCreation, binding *Binding) error {
	return s.access(ctx, func(doc *bindingDocument) (bool, error) {
		key := scope.Key()
		if current, ok := doc.Bindings[key]; ok {
			if binding != nil && current.ConnectionID == binding.ConnectionID && current.Project.ID == binding.Project.ID && current.TeamID == binding.TeamID {
				return false, nil
			}
			return false, Fail("project_bound")
		}
		if doc.Creations[key] != creation {
			return false, Fail("project_create_pending")
		}
		delete(doc.Creations, key)
		if binding != nil {
			doc.Bindings[key] = *binding
		}
		return true, nil
	})
}
