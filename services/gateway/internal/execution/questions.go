package execution

import (
	"slices"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

type questionReport struct {
	JobID    string                `json:"jobId"`
	Attempt  string                `json:"attemptId"`
	Session  string                `json:"session"`
	Question api.ExecutionQuestion `json:"question"`
	Waiting  bool                  `json:"waiting"`
}

func (p questionReport) valid() bool {
	return api.ValidExecutionID(p.JobID) && validSession(p.Attempt) && validSession(p.Session) &&
		p.Question.Valid() && p.Question.Answer == ""
}

func (s *Store) exchangeQuestion(workerID string, update questionReport) (api.ExecutionQuestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[update.JobID]
	if !ok || job.Input.Kind != api.LinearWorkflow || job.WorkerID != workerID || job.Attempt != update.Attempt ||
		job.Session != update.Session || s.workers[workerID].Session != update.Session {
		return api.ExecutionQuestion{}, errNotFound
	}
	question := update.Question
	last := len(job.Questions) - 1
	if last >= 0 && job.Questions[last].ID == question.ID {
		if !activeAssignment(job.State) && !(job.releasesOnQuestion() && job.State == api.ExecutionQueued && update.Waiting) {
			return api.ExecutionQuestion{}, errNotFound
		}
		stored := job.Questions[last]
		if stored.Text != question.Text || (!update.Waiting && stored.Answer == "") {
			return api.ExecutionQuestion{}, errQuestionConflict
		}
		if !update.Waiting && job.State == api.ExecutionWaitingForAnswer {
			job.State, job.UpdatedAt = api.ExecutionRunning, time.Now().UTC()
			return stored, s.save(job)
		}
		return stored, nil
	}
	if !activeAssignment(job.State) || !update.Waiting || (last >= 0 && job.Questions[last].Answer == "") ||
		slices.ContainsFunc(job.Questions, func(q api.ExecutionQuestion) bool { return q.ID == question.ID }) {
		return api.ExecutionQuestion{}, errQuestionConflict
	}
	job.Questions = append(slices.Clone(job.Questions), question)
	job.State, job.UpdatedAt = api.ExecutionWaitingForAnswer, time.Now().UTC()
	return question, s.save(job)
}

func (s *Store) answer(id string, input api.ExecutionAnswer) (api.ExecutionQuestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return api.ExecutionQuestion{}, errNotFound
	}
	index := slices.IndexFunc(job.Questions, func(question api.ExecutionQuestion) bool {
		return question.ID == input.QuestionID
	})
	if index < 0 {
		return api.ExecutionQuestion{}, errQuestionConflict
	}
	question := job.Questions[index]
	if question.Answer == input.Answer {
		return question, nil
	}
	if job.State != api.ExecutionWaitingForAnswer || index != len(job.Questions)-1 || question.Answer != "" {
		return api.ExecutionQuestion{}, errQuestionConflict
	}
	question.Answer = input.Answer
	job.Questions = slices.Clone(job.Questions)
	job.Questions[index] = question
	job.UpdatedAt = time.Now().UTC()
	if job.releasesOnQuestion() {
		job.State = api.ExecutionQueued
	}
	return question, s.save(job)
}
