package execution

import (
	"cmp"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
)

const LocalWorkerID = "local"
const heartbeatTimeout = 20 * time.Second

var (
	errNotFound         = errors.New("execution_not_found")
	errWorker           = errors.New("worker_not_found")
	errRepository       = errors.New("worker_repository_unavailable")
	errKind             = errors.New("worker_kind_unavailable")
	errModel            = errors.New("worker_model_unavailable")
	errWorkflow         = errors.New("worker_workflow_unavailable")
	errConflict         = errors.New("execution_conflict")
	errQuestionConflict = errors.New("execution_question_conflict")
)

type assignment struct {
	api.Execution
	Workflow      *api.JobSummary `json:"workflow,omitempty"`
	Attempt       string          `json:"attemptId,omitempty"`
	Session       string          `json:"session,omitempty"`
	CanAsk        bool            `json:"canAsk,omitempty"`
	CanRequeue    bool            `json:"canRequeue,omitempty"`
	LogsTruncated bool            `json:"logsTruncated,omitempty"`
	PreviousLogs  []byte          `json:"previousLogs,omitempty"`
}

type registeredWorker struct {
	api.ExecutionWorker
	TokenHash string `json:"tokenHash,omitempty"`
	Session   string `json:"-"`
}

type Store struct {
	mu      sync.Mutex
	root    string
	lock    *flock.Flock
	jobs    map[string]assignment
	workers map[string]registeredWorker
	invites map[string]time.Time
}

func Open(root, localName string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(root, "jobs"), 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(root, "lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errConflict
	}
	s := &Store{root: root, lock: lock, jobs: map[string]assignment{}, workers: map[string]registeredWorker{}, invites: map[string]time.Time{}}
	if err := s.load(); err != nil {
		_ = lock.Close()
		return nil, err
	}
	s.workers[LocalWorkerID] = registeredWorker{ExecutionWorker: api.ExecutionWorker{ID: LocalWorkerID, Name: localName}}
	return s, nil
}

func (s *Store) Close() error { return s.lock.Close() }

func readJSON(path string, into any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}

func writeJSON(path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, body, 0600)
}

func (s *Store) load() error {
	if err := readJSON(filepath.Join(s.root, "workers.json"), &s.workers); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for id, worker := range s.workers {
		worker.LastSeen = time.Time{}
		s.workers[id] = worker
	}
	entries, err := os.ReadDir(filepath.Join(s.root, "jobs"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var job assignment
		if err := readJSON(filepath.Join(s.root, "jobs", entry.Name()), &job); err != nil {
			return err
		}
		if job.ID+".json" != entry.Name() || job.ID != digest(job.RequestID)[:32] {
			return errConflict
		}
		if job.State == api.ExecutionWaitingToContinue && job.Progress != nil && job.Progress.ContinueRequested {
			// An already accepted continuation from the previous scheduler may be running.
			job.State = api.ExecutionRunning
			if err := s.save(job); err != nil {
				return err
			}
			continue
		}
		s.jobs[job.ID] = job
	}
	return nil
}

func digest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func (s *Store) save(job assignment) error {
	if err := writeJSON(filepath.Join(s.root, "jobs", job.ID+".json"), job); err != nil {
		return err
	}
	s.jobs[job.ID] = job
	return nil
}

func (s *Store) submit(input api.ExecutionSubmission) (api.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := digest(input.RequestID)[:32]
	if job, ok := s.jobs[id]; ok {
		if !reflect.DeepEqual(job.ExecutionSubmission, input) {
			return api.Execution{}, errConflict
		}
		return s.view(job), nil
	}
	worker, ok := s.workers[input.WorkerID]
	if !ok {
		return api.Execution{}, errWorker
	}
	if input.Input.Kind == api.LinearWorkflow && input.WorkerID != LocalWorkerID && !slices.ContainsFunc(worker.LinearProjects, func(project api.LinearProject) bool {
		return projectMatches(project, input.Input.Linear.Project)
	}) {
		return api.Execution{}, errWorkflow
	}
	// An offline machine's capabilities will be confirmed when it reconnects.
	if online(worker) {
		if input.Input.Kind != api.LinearWorkflow && !slices.ContainsFunc(worker.Repositories, input.Input.Repository.Matches) {
			return api.Execution{}, errRepository
		}
		if input.Input.Kind == api.CodexTask && worker.Models == nil {
			return api.Execution{}, errKind
		}
		if worker.Models != nil && !supportsModel(*worker.Models, input.Input.Codex) {
			return api.Execution{}, errModel
		}
	}
	now := time.Now().UTC()
	job := assignment{Execution: api.Execution{ID: id, ExecutionSubmission: input, State: api.ExecutionQueued, CreatedAt: now, UpdatedAt: now}}
	return s.view(job), s.save(job)
}

func online(worker registeredWorker) bool {
	return time.Since(worker.LastSeen) <= heartbeatTimeout
}

func (s *Store) view(job assignment) api.Execution {
	result := job.Execution
	result.CanRevise = job.canRevise()
	result.Subtasks = slices.Clone(result.Subtasks)
	result.Questions = slices.Clone(result.Questions)
	if result.Progress != nil {
		progress := *result.Progress
		result.Progress = &progress
	}
	worker := s.workers[job.WorkerID]
	switch result.State {
	case api.ExecutionQueued:
		if !online(worker) {
			result.Reason = "worker_offline"
		}
	case api.ExecutionRunning:
		if !online(worker) || worker.Session != job.Session {
			result.State, result.Reason = api.ExecutionUnknown, "worker_disconnected"
		}
	}
	return result
}

func (s *Store) job(id string) (api.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, found := s.jobs[id]
	if !found {
		return api.Execution{}, errNotFound
	}
	return s.view(job), nil
}

func (s *Store) list() []api.Execution {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]api.Execution, 0, len(s.jobs))
	for _, job := range s.jobs {
		item := s.view(job)
		item.Input.Prompt, item.Result = "", nil
		item.Questions = nil
		item.Subtasks = nil
		if item.State != api.ExecutionWaitingToContinue {
			item.Progress = nil
		}
		if item.State == api.ExecutionWaitingForAnswer && len(job.Questions) != 0 {
			item.Questions = []api.ExecutionQuestion{job.Questions[len(job.Questions)-1]}
		}
		jobs = append(jobs, item)
	}
	slices.SortFunc(jobs, func(a, b api.Execution) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return jobs
}

func (s *Store) workersList() []api.ExecutionWorker {
	s.mu.Lock()
	defer s.mu.Unlock()
	workers := make([]api.ExecutionWorker, 0, len(s.workers))
	for _, worker := range s.workers {
		item := worker.ExecutionWorker
		item.Online = online(worker)
		item.Busy = item.Online && slicesContainsRunning(s.jobs, worker.ID, worker.Session)
		workers = append(workers, item)
	}
	slices.SortFunc(workers, func(a, b api.ExecutionWorker) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return workers
}

func slicesContainsRunning(jobs map[string]assignment, workerID, session string) bool {
	for _, job := range jobs {
		if job.WorkerID == workerID && job.Session == session && job.reservesWorker() {
			return true
		}
	}
	return false
}

func activeAssignment(state string) bool {
	return state == api.ExecutionRunning || state == api.ExecutionWaitingForAnswer
}

func (job assignment) reservesWorker() bool {
	return activeAssignment(job.State) && !(job.State == api.ExecutionWaitingForAnswer && job.releasesOnQuestion())
}

func (job assignment) releasesOnQuestion() bool {
	return job.Input.Linear != nil && (len(job.Input.Linear.SubtaskOrder) > 0 || job.Input.Linear.Revision != nil)
}

func (s *Store) heartbeat(workerID, session string, inventory api.ExecutionWorker) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	worker, ok := s.workers[workerID]
	if !ok {
		return errWorker
	}
	changed := !slices.Equal(worker.Repositories, inventory.Repositories) ||
		!reflect.DeepEqual(worker.Models, inventory.Models) || !reflect.DeepEqual(worker.LinearProjects, inventory.LinearProjects)
	worker.Repositories, worker.Models, worker.LinearProjects = inventory.Repositories, inventory.Models, inventory.LinearProjects
	if workerID != LocalWorkerID && changed {
		if err := s.saveWorker(worker); err != nil {
			return err
		}
	}
	worker.LastSeen, worker.Session = time.Now().UTC(), session
	s.workers[workerID] = worker
	return nil
}

func (s *Store) saveWorker(worker registeredWorker) error {
	workers := maps.Clone(s.workers)
	delete(workers, LocalWorkerID)
	workers[worker.ID] = worker
	return writeJSON(filepath.Join(s.root, "workers.json"), workers)
}

func (s *Store) claim(workerID, session string) (*assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	worker, ok := s.workers[workerID]
	if !ok || worker.Session != session || !online(worker) {
		return nil, errWorker
	}
	var next *assignment
	for _, job := range s.jobs {
		if job.WorkerID != workerID {
			continue
		}
		if job.reservesWorker() {
			job.CanAsk = job.Input.Kind == api.LinearWorkflow
			job.CanRequeue = job.Input.Kind == api.LinearWorkflow
			if job.Session == session {
				return s.claimReply(job)
			}
			if job.Input.Kind == api.LinearWorkflow {
				job.Session = session
				job.UpdatedAt = time.Now().UTC()
				if err := s.save(job); err != nil {
					return nil, err
				}
				return s.claimReply(job)
			}
		}
		// UpdatedAt is the enqueue time for both new and requeued jobs.
		if job.State == api.ExecutionQueued && (next == nil ||
			cmp.Or(job.UpdatedAt.Compare(next.UpdatedAt), cmp.Compare(job.ID, next.ID)) < 0) {
			next = &job
		}
	}
	if next == nil {
		return nil, nil
	}
	next.Session, next.Attempt = session, rand.Text()
	next.CanAsk = next.Input.Kind == api.LinearWorkflow
	next.CanRequeue = next.Input.Kind == api.LinearWorkflow
	next.State, next.UpdatedAt = api.ExecutionRunning, time.Now().UTC()
	if err := s.save(*next); err != nil {
		return nil, err
	}
	return s.claimReply(*next)
}

func (s *Store) claimReply(job assignment) (*assignment, error) {
	job.Workflow = nil
	if job.Progress != nil || job.releasesOnQuestion() && len(job.Questions) > 0 {
		logs, err := s.readLogs(job.ID)
		if err != nil {
			return nil, err
		}
		job.PreviousLogs = logs
	}
	return &job, nil
}

type report struct {
	JobID    string                 `json:"jobId"`
	Attempt  string                 `json:"attemptId"`
	Result   api.ExecutionResult    `json:"result"`
	Logs     []byte                 `json:"logs,omitempty"`
	Progress *api.ExecutionProgress `json:"progress,omitempty"`
}

func (s *Store) complete(workerID string, done report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[done.JobID]
	if !ok || job.WorkerID != workerID || job.Attempt != done.Attempt || done.Attempt == "" {
		return errNotFound
	}
	if job.Result != nil {
		return nil
	}
	if done.Progress != nil {
		return s.pause(job, done)
	}
	if !activeAssignment(job.State) {
		return errNotFound
	}
	if err := s.saveLogs(job.ID, done.Logs); err != nil {
		return err
	}
	job.State = api.ExecutionSucceeded
	if done.Result.Error != "" || (done.Result.ExitCode != nil && *done.Result.ExitCode != 0) {
		job.State = api.ExecutionFailed
	}
	job.Result, job.UpdatedAt = &done.Result, time.Now().UTC()
	if done.Result.Subtasks != nil {
		job.Subtasks = nil
	}
	return s.save(job)
}

func (s *Store) saveLogs(id string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	path := filepath.Join(s.root, "jobs", id+".log")
	info, err := os.Stat(path)
	if err == nil && info.Size() >= int64(len(data)) {
		return nil
	}
	return fsutil.WriteAtomic(path, data, 0600)
}

func (s *Store) logs(id string, offset int) (api.ExecutionLogs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return api.ExecutionLogs{}, errNotFound
	}
	data, err := s.readLogs(id)
	if err != nil {
		return api.ExecutionLogs{}, err
	}
	offset = min(offset, len(data))
	return api.ExecutionLogs{Data: data[offset:], Next: len(data), State: s.view(job).State,
		Truncated: job.LogsTruncated || (job.Result != nil && job.Result.LogsTruncated)}, nil
}

func (s *Store) readLogs(id string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "jobs", id+".log"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}
