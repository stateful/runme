package kernel

import (
	"sync"
)

type sessionsContainer struct {
	mu       sync.RWMutex
	sessions []*session
}

func (c *sessionsContainer) AddSession(s *session) {
	c.mu.Lock()
	c.sessions = append(c.sessions, s)
	c.mu.Unlock()
}

func (c *sessionsContainer) FindSession(id string) *session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.sessions {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (c *sessionsContainer) DeleteSession(session *session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for idx, s := range c.sessions {
		if s.id == session.id {
			if idx == len(c.sessions)-1 {
				c.sessions = c.sessions[:idx]
			} else {
				c.sessions = append(c.sessions[:idx], c.sessions[idx+1:]...)
			}
			return
		}
	}
}

func (c *sessionsContainer) Sessions() []*session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions
}
