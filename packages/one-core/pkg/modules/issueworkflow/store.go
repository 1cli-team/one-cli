package issueworkflow

import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/gofrs/flock"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type stage string

const (
	queued            stage = "queued"
	running           stage = "running"
	waitingForAnswer  stage = api.ExecutionWaitingForAnswer
	waitingToContinue stage = api.ExecutionWaitingToContinue
	verifying         stage = "verifying"
	preparingSubtask  stage = "preparing_subtask"
	committingSubtask stage = "committing_subtask"
	publishing        stage = "publishing"
	opening           stage = "opening"
	toReview          stage = "to_review"
	review            stage = "review"
	toDone            stage = "to_done"
	done              stage = "done"
	failed            stage = "failed"
)

type Job struct {
	ID             string                 `json:"id"`
	ProjectID      string                 `json:"projectId"`
	Issue          tracking.Issue         `json:"issue"`
	Project        Project                `json:"project"`
	Codex          api.CodexOptions       `json:"codex,omitzero"`
	AskUser        bool                   `json:"askUser,omitempty"`
	TrackProgress  bool                   `json:"trackProgress,omitempty"`
	Question       *api.ExecutionQuestion `json:"question,omitempty"`
	Progress       *api.ExecutionProgress `json:"progress,omitempty"`
	Stage          stage                  `json:"stage"`
	Created        time.Time              `json:"created"`
	Updated        time.Time              `json:"updated,omitzero"`
	Worktree       string                 `json:"worktree,omitempty"`
	CheckDir       string                 `json:"checkDir,omitempty"`
	BaseSHA        string                 `json:"baseSha,omitempty"`
	Branch         string                 `json:"branch,omitempty"`
	ThreadID       string                 `json:"threadId,omitempty"`
	TurnID         string                 `json:"turnId,omitempty"`
	CodexResponse  string                 `json:"codexResponse,omitempty"`
	HeadSHA        string                 `json:"headSha,omitempty"`
	OpenSince      time.Time              `json:"openSince,omitzero"`
	PR             PullRequest            `json:"pr"`
	LastError      string                 `json:"lastError,omitempty"`
	RepositoryRuns []RepositoryRun        `json:"repositoryRuns,omitempty"`
	Subtasks       []SubtaskRun           `json:"subtasks,omitempty"`
	Revision       *Revision              `json:"revision,omitempty"`
	readRoots      []string
}

type RepositoryRun struct {
	Worktree  string      `json:"worktree"`
	CheckDir  string      `json:"checkDir,omitempty"`
	Base      string      `json:"base,omitempty"`
	BaseSHA   string      `json:"baseSha,omitempty"`
	Branch    string      `json:"branch,omitempty"`
	HeadSHA   string      `json:"headSha,omitempty"`
	Unchanged bool        `json:"unchanged,omitempty"`
	OpenSince time.Time   `json:"openSince,omitzero"`
	PR        PullRequest `json:"pr"`
}

func (j Job) clone() Job {
	j.RepositoryRuns = slices.Clone(j.RepositoryRuns)
	j.Subtasks = slices.Clone(j.Subtasks)
	if j.Question != nil {
		question := *j.Question
		j.Question = &question
	}
	if j.Progress != nil {
		progress := *j.Progress
		j.Progress = &progress
	}
	j.readRoots = nil
	return j
}

func (j Job) readyToResume() bool {
	return (j.Stage == waitingForAnswer && j.Question != nil && j.Question.Answer != "") ||
		(j.Stage == waitingToContinue && j.Progress != nil && j.Progress.ContinueRequested)
}

func (j Job) status() api.Status {
	switch j.Stage {
	case review, done:
		return api.Submitted
	case failed:
		return api.Failed
	default:
		return api.Processing
	}
}

type Store struct {
	mu   sync.Mutex
	dir  string
	lock *flock.Flock
	jobs map[string]Job
}

func OpenStore(dir string) (*Store, error) {
	jobsDir := filepath.Join(dir, "jobs")
	if err := os.MkdirAll(jobsDir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "gateway.lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, i18n.Errorf("linear.cli.workflow_lock", dir)
	}
	store := &Store{dir: jobsDir, lock: lock, jobs: make(map[string]Job)}
	if err := store.load(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.lock.Close() }

func (s *Store) load() error {
	files, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.dir, file.Name()))
		if err != nil {
			return err
		}
		var job Job
		if err := json.Unmarshal(body, &job); err != nil {
			return fmt.Errorf("load job %s: %w", file.Name(), err)
		}
		if job.ID+".json" != file.Name() || !uuidPattern.MatchString(job.ID) ||
			!uuidPattern.MatchString(job.ProjectID) || !uuidPattern.MatchString(job.Issue.ID) {
			return fmt.Errorf("invalid job record %s", file.Name())
		}
		if len(job.RepositoryRuns) != 0 && len(job.RepositoryRuns) != len(job.Project.Repositories) {
			return fmt.Errorf("invalid repository progress in job record %s", file.Name())
		}
		key := job.recordKey()
		if _, ok := s.jobs[key]; ok {
			return fmt.Errorf("duplicate job for %s", key)
		}
		s.jobs[key] = job
	}
	return nil
}

func jobKey(project, issue string) string { return project + "/" + issue }

func runKey(project, issue, comment string) string {
	key := jobKey(project, issue)
	if comment != "" {
		key += "/" + comment
	}
	return key
}

func (j Job) recordKey() string {
	return runKey(j.ProjectID, j.Issue.ID, j.revisionCommentID())
}

func (s *Store) get(project, issue string) (Job, bool) {
	return s.getRun(project, issue, "")
}

func (s *Store) getRun(project, issue, comment string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[runKey(project, issue, comment)]
	return job.clone(), ok
}

func (s *Store) findID(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.ID == id {
			return job.clone(), true
		}
	}
	return Job{}, false
}

func (s *Store) enqueue(job Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.jobs[job.recordKey()]; ok {
		return existing.clone(), nil
	}
	if job.Revision != nil {
		for _, existing := range s.jobs {
			if existing.Revision != nil && existing.Issue.ID == job.Issue.ID && existing.ProjectID == job.ProjectID &&
				existing.Stage != failed && existing.Stage != review {
				return Job{}, revisionInProgress
			}
		}
	}
	// Use a stable UUID basename for filesystem recovery and stored job validation.
	var id [16]byte
	rand.Read(id[:])
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	job.ID = fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	job.Stage, job.Created = queued, time.Now().UTC()
	return job, s.saveLocked(job)
}

func (s *Store) save(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(job)
}

func (s *Store) answer(projectID, issueID, commentID string, input api.ExecutionAnswer) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[runKey(projectID, issueID, commentID)]
	if !ok {
		return http.StatusNotFound, errors.New("workflow job not found")
	}
	if job.Question == nil || job.Question.ID != input.QuestionID {
		return http.StatusConflict, errors.New("question does not match the workflow job")
	}
	question := *job.Question
	if question.Answer == input.Answer {
		return http.StatusOK, nil
	}
	if job.Stage != waitingForAnswer || question.Answer != "" {
		return http.StatusConflict, errors.New("question is no longer waiting for an answer")
	}
	question.Answer = input.Answer
	job.Question = &question
	if err := s.saveLocked(job); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func (s *Store) requestContinue(projectID, issueID, commentID, progressID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[runKey(projectID, issueID, commentID)]
	if !ok {
		return http.StatusNotFound, errors.New("workflow job not found")
	}
	if job.Progress == nil || job.Progress.ID != progressID ||
		(job.Stage != waitingToContinue && !job.Progress.ContinueRequested) {
		return http.StatusConflict, errors.New("progress does not match a paused turn")
	}
	if job.Progress.ContinueRequested {
		return http.StatusOK, nil
	}
	progress := *job.Progress
	progress.ContinueRequested = true
	job.Progress = &progress
	if err := s.saveLocked(job); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func (s *Store) saveLocked(job Job) error {
	job.Updated = time.Now().UTC()
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if err := fsutil.WriteAtomic(filepath.Join(s.dir, job.ID+".json"), data, 0600); err != nil {
		return err
	}
	s.jobs[job.recordKey()] = job.clone()
	return nil
}

func (s *Store) list() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job.clone())
	}
	slices.SortFunc(jobs, func(a, b Job) int {
		return cmp.Or(a.Created.Compare(b.Created), cmp.Compare(a.ID, b.ID))
	})
	return jobs
}

func (s *Store) counts() api.QueueCounts {
	s.mu.Lock()
	defer s.mu.Unlock()
	var counts api.QueueCounts
	for _, job := range s.jobs {
		if job.Revision != nil {
			continue
		}
		switch job.Stage {
		case queued:
			counts.Queued++
		case running, waitingForAnswer, waitingToContinue, verifying, preparingSubtask, committingSubtask, publishing, opening, toReview, toDone:
			counts.Active++
		case review:
			counts.Review++
		case done:
			counts.Completed++
		case failed:
			counts.Failed++
		}
	}
	return counts
}
