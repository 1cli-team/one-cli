package tasks

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// This is timestamp freshness, not an artifact cache. Missing inputs or outputs
// and unreadable paths force execution. All output patterns must exist.
func fresh(task Task) bool {
	if len(task.Sources) == 0 || len(task.Outputs) == 0 {
		return false
	}
	var newest time.Time
	for _, pattern := range task.Sources {
		times, err := patternTimes(task.Directory, pattern)
		if err != nil || len(times) == 0 {
			return false
		}
		for _, mtime := range times {
			if mtime.After(newest) {
				newest = mtime
			}
		}
	}
	if info, err := os.Stat(task.Source); err == nil && info.ModTime().After(newest) {
		newest = info.ModTime()
	}
	for _, pattern := range task.Outputs {
		times, err := patternTimes(task.Directory, pattern)
		if err != nil || len(times) == 0 {
			return false
		}
		for _, mtime := range times {
			if mtime.Before(newest) {
				return false
			}
		}
	}
	return true
}
func patternTimes(directory, pattern string) ([]time.Time, error) {
	absolute := filepath.Clean(filepath.Join(directory, pattern))
	if filepath.IsAbs(pattern) {
		absolute = filepath.Clean(pattern)
	}
	root := absolute
	for strings.ContainsAny(root, "*?[") {
		root = filepath.Dir(root)
	}
	expression, err := globExpression(filepath.ToSlash(absolute))
	if err != nil {
		return nil, err
	}
	var times []time.Time
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if expression.MatchString(filepath.ToSlash(path)) || !strings.ContainsAny(absolute, "*?[") {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			times = append(times, info.ModTime())
		}
		return nil
	})
	return times, err
}
func globExpression(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				return nil, fs.ErrInvalid
			}
			end += i + 1
			group := pattern[i+1 : end]
			if strings.HasPrefix(group, "!") {
				group = "^" + group[1:]
			}
			b.WriteString("[" + group + "]")
			i = end
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
