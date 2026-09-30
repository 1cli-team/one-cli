package taskui

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Only offsets and task membership stay in memory. Log bodies live in a private
// session file, so scrolling never evicts earlier output. close removes it.
type logEntry struct {
	offset int64
	size   int
}
type logStore struct {
	closed       bool
	mu           sync.Mutex
	file         *os.File
	names        []string
	prefixStyles map[string]uv.Style
	entries      []logEntry
	tasks        map[string][]int
	observed     map[string]bool
	writers      []*logWriter
	pending      map[*logWriter]logLine
	size         int64
	err          error
}
type logLine struct{ task, text string }

func newLogStore(names []string) (*logStore, error) {
	f, err := os.CreateTemp("", "one-task-logs-*")
	if err != nil {
		return nil, err
	}
	return &logStore{file: f, names: names, prefixStyles: taskPrefixStyles(names), tasks: map[string][]int{}, observed: map[string]bool{}, pending: map[*logWriter]logLine{}}, nil
}
func (s *logStore) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	_ = s.file.Close()
	_ = os.Remove(s.file.Name())
}
func (s *logStore) failure() error { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *logStore) add(line logLine) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	n, err := s.file.WriteAt([]byte(line.text), s.size)
	if err == nil && n != len(line.text) {
		err = io.ErrShortWrite
	}
	if err != nil {
		s.err = err
		return err
	}
	id := len(s.entries)
	s.entries = append(s.entries, logEntry{s.size, n})
	s.size += int64(n)
	s.tasks[line.task] = append(s.tasks[line.task], id)
	s.observed[line.task] = true
	return nil
}
func (s *logStore) count(task string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task == "" {
		return len(s.entries)
	}
	return len(s.tasks[task])
}
func (s *logStore) id(task string, ordinal int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task == "" {
		return ordinal
	}
	if ordinal < 0 || ordinal >= len(s.tasks[task]) {
		return -1
	}
	return s.tasks[task][ordinal]
}
func (s *logStore) read(id int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id < 0 || id >= len(s.entries) || s.err != nil || s.closed {
		return ""
	}
	e := s.entries[id]
	b := make([]byte, e.size)
	if _, err := s.file.ReadAt(b, e.offset); err != nil {
		s.err = err
		return ""
	}
	return string(b)
}
func (s *logStore) hasOutput(task string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.observed[task]
}
func (s *logStore) setPending(w *logWriter, line logLine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if line.text == "" {
		delete(s.pending, w)
	} else {
		s.pending[w] = line
		s.observed[line.task] = true
	}
}
func (s *logStore) unfinished(task string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lines []string
	// The order is stable even when stdout and stderr both have incomplete lines.
	for _, w := range s.writers {
		if line, ok := s.pending[w]; ok && (task == "" || task == line.task) {
			lines = append(lines, line.text)
		}
	}
	return lines
}
func (s *logStore) search(ctx context.Context, task, query string, from, to int) []int {
	var ids []int
	query = strings.ToLower(query)
	for i := from; i < to; i++ {
		if ctx.Err() != nil {
			return nil
		}
		id := s.id(task, i)
		if strings.Contains(strings.ToLower(ansi.Strip(s.read(id))), query) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *logStore) ids(task string) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task != "" {
		return append([]int(nil), s.tasks[task]...)
	}
	ids := make([]int, len(s.entries))
	for i := range ids {
		ids[i] = i
	}
	return ids
}
