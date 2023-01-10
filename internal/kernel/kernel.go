package kernel

import (
	"sync"
)

type kernel struct {
	mu       sync.RWMutex
	sessions []*Session
}

func (k *kernel) AddSession(s *Session) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.sessions = append(k.sessions, s)
}

func (k *kernel) FindSession(id string) *Session {
	k.mu.RLock()
	defer k.mu.RUnlock()
	for _, s := range k.sessions {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (k *kernel) DeleteSession(session *Session) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for idx, s := range k.sessions {
		if s.id == session.id {
			if idx == len(k.sessions)-1 {
				k.sessions = k.sessions[:idx]
			} else {
				k.sessions = append(k.sessions[:idx], k.sessions[idx+1:]...)
			}
			return
		}
	}
}

func (k *kernel) Sessions() []*Session {
	return k.sessions
}
