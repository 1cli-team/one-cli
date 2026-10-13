// Package desktopcontrol defines the private, versioned host lifecycle protocol.
// Business requests never travel on this channel.
package desktopcontrol

import (
	"sync"
)

const Protocol = 1
const API = 1

type Identity struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Channel  string `json:"channel"`
	Protocol int    `json:"protocol"`
	API      int    `json:"api"`
}

type Bootstrap struct {
	Type       string `json:"type"`
	Protocol   int    `json:"protocol"`
	Generation string `json:"generation"`
	Token      string `json:"token"`
	Origin     string `json:"origin"`
}

type Session struct {
	Bootstrap
	Identity Identity
	mu       sync.Mutex
	stopping bool
	active   int
}

// Begin and Quiesce share the same lock: no new mutation can slip through
// after the host decides to install an update or close the application.
func (s *Session) Begin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		return false
	}
	s.active++
	return true
}
func (s *Session) End()         { s.mu.Lock(); s.active--; s.mu.Unlock() }
func (s *Session) Quiesce() int { s.mu.Lock(); defer s.mu.Unlock(); s.stopping = true; return s.active }
func (s *Session) Resume()      { s.mu.Lock(); s.stopping = false; s.mu.Unlock() }
