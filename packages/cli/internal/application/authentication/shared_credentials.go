// Package authentication coordinates the saved login session and shared storage.
// The platform session package remains independent of environment operations.
package authentication

import (
	"context"
	"errors"
	"sync"
	"time"

	environment "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

const preparationTimeout = 2 * time.Minute

type SharedCredentialsState struct {
	Status   string                      `json:"status"`
	Location *environment.GlobalLocation `json:"location,omitempty"`
	Error    string                      `json:"error,omitempty"`
}

// PrepareAfterLogin is also used by the CLI. A storage failure does not undo
// authentication, and no secret value or session token is part of its result.
func PrepareAfterLogin(ctx context.Context, info session.Info) SharedCredentialsState {
	ctx, cancel := context.WithTimeout(ctx, preparationTimeout)
	defer cancel()
	expected, err := session.Require()
	if err == nil && (!sameIdentity(expected.Info, info) || !expected.ExpiresAt.Equal(info.ExpiresAt)) {
		err = i18n.Errorf("auth.session_changed")
	}
	var location *environment.GlobalLocation
	if err == nil {
		done := make(chan struct{})
		go func() { defer close(done); cancelOnSessionChange(ctx, expected, cancel) }()
		defer func() { cancel(); <-done }()
		location, err = environment.PrepareSharedCredentials(ctx, expected)
	}
	return preparationResult(location, err)
}

func cancelOnSessionChange(ctx context.Context, expected *session.Session, cancel context.CancelFunc) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := session.Require()
			if err != nil || !sameIdentity(current.Info, expected.Info) || current.Token != expected.Token {
				cancel()
				return
			}
		}
	}
}

func preparationResult(location *environment.GlobalLocation, err error) SharedCredentialsState {
	if err != nil {
		return SharedCredentialsState{Status: "failed", Error: preparationErrorMessage(err)}
	}
	return SharedCredentialsState{Status: "ready", Location: location}
}

func preparationErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return i18n.T("auth.shared_timeout")
	}
	return err.Error()
}

func sameIdentity(a, b session.Info) bool {
	return a.SiteURL == b.SiteURL && a.UserID == b.UserID && a.OrganizationID == b.OrganizationID
}

type SharedCredentialsService struct {
	mu         sync.Mutex
	work       sync.WaitGroup
	expected   *session.Session
	state      SharedCredentialsState
	failure    error
	generation uint64
	cancel     context.CancelFunc
}

func NewSharedCredentialsService() *SharedCredentialsService {
	return &SharedCredentialsService{}
}

// Run starts initialization for an existing session and observes CLI login,
// logout and token changes. Failed preparations wait for an explicit retry.
// Polling the session is local; unchanged identities cause no remote requests.
func (s *SharedCredentialsService) Run(ctx context.Context) {
	defer func() { s.Stop(); s.work.Wait() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if current, err := session.Require(); err == nil {
			s.start(ctx, current, false)
		} else {
			s.Stop()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Retry is an explicit mutation invoked by POST, never by a status GET.
func (s *SharedCredentialsService) Retry(ctx context.Context) (*SharedCredentialsState, error) {
	current, err := session.Require()
	if err != nil {
		return nil, err
	}
	s.start(ctx, current, true)
	return s.State(current.Info), nil
}

func (s *SharedCredentialsService) start(ctx context.Context, current *session.Session, retry bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	unchanged := s.expected != nil && sameIdentity(s.expected.Info, current.Info) && s.expected.Token == current.Token
	if unchanged && (!retry || s.state.Status == "preparing") {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.generation++
	generation := s.generation
	ctx, cancel := context.WithTimeout(ctx, preparationTimeout)
	s.cancel, s.expected = cancel, current
	s.state = SharedCredentialsState{Status: "preparing"}
	s.failure = nil
	s.work.Add(1)
	go func() {
		defer s.work.Done()
		defer cancel()
		location, err := environment.PrepareSharedCredentials(ctx, current)
		result := preparationResult(location, err)
		// Storage commits already check the session. Check it again before
		// publishing a result, including failures returned by an old task.
		publishErr := session.WithUnchanged(current, func() error {
			s.mu.Lock()
			defer s.mu.Unlock()
			if generation == s.generation {
				s.state = result
				s.failure = err
			}
			return nil
		})
		if publishErr != nil {
			s.mu.Lock()
			defer s.mu.Unlock()
			latest, err := session.Require()
			if generation == s.generation && err == nil && sameIdentity(latest.Info, current.Info) && latest.Token == current.Token {
				s.state = preparationResult(nil, publishErr)
				s.failure = publishErr
			}
		}
	}()
}

// State is read-only and never exposes another identity's location or errors.
func (s *SharedCredentialsService) State(info session.Info) *SharedCredentialsState {
	if !info.LoggedIn {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expected == nil || !sameIdentity(s.expected.Info, info) || !s.expected.ExpiresAt.Equal(info.ExpiresAt) {
		return &SharedCredentialsState{Status: "preparing"}
	}
	state := s.state
	if s.failure != nil {
		state.Error = preparationErrorMessage(s.failure)
	}
	if state.Location != nil {
		location := *state.Location
		state.Location = &location
	}
	return &state
}

func (s *SharedCredentialsService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	s.generation++
	s.expected, s.cancel = nil, nil
	s.state = SharedCredentialsState{}
	s.failure = nil
}
