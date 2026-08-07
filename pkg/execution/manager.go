package execution

import (
	"fmt"
	"regexp"
	"sync"
	"time"
)

// SessionManager handles the lifecycle of msh sessions.
// It provides thread-safe access to sessions and automatically
// cleans up sessions that have been idle for too long.
type SessionManager struct {
	sessions    map[string]*SessionWrapper
	mu          sync.RWMutex
	timeout     time.Duration
	maxSessions int
}

// validSessionID only allows alphanumeric, hyphens, and underscores.
var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,128}$`)

// SessionWrapper wraps a Session with metadata for cleanup.
type SessionWrapper struct {
	Session    *Session
	LastActive time.Time
}

// NewSessionManager creates a new session manager with the given idle timeout.
func NewSessionManager(idleTimeout time.Duration) *SessionManager {
	if idleTimeout == 0 {
		idleTimeout = 30 * time.Minute // default 30 minutes
	}
	return &SessionManager{
		sessions:    make(map[string]*SessionWrapper),
		timeout:     idleTimeout,
		maxSessions: 1000, // prevent memory exhaustion
	}
}

// GetOrCreateSession retrieves an existing session by ID or creates a new one.
// If the provided ID is empty, a new session is created with a generated ID.
// The baseCwd is used as the starting directory for new sessions.
func (sm *SessionManager) GetOrCreateSession(id string, baseCwd string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate session ID format to prevent injection
	if id != "" {
		if !validSessionID.MatchString(id) {
			return nil, fmt.Errorf("invalid session ID format")
		}
		if wrapper, exists := sm.sessions[id]; exists {
			wrapper.LastActive = time.Now()
			return wrapper.Session, nil
		}
	}

	// Enforce max sessions to prevent memory exhaustion
	if len(sm.sessions) >= sm.maxSessions {
		return nil, fmt.Errorf("maximum session limit (%d) reached", sm.maxSessions)
	}

	// Create new session
	session, err := NewSession(baseCwd)
	if err != nil {
		return nil, err
	}

	// If client provided a specific ID, use it instead of the generated one
	if id != "" {
		session.ID = id
	}

	sm.sessions[session.ID] = &SessionWrapper{
		Session:    session,
		LastActive: time.Now(),
	}

	return session, nil
}

// CleanupIdleSessions removes sessions that have not been accessed
// within the manager's timeout duration.
func (sm *SessionManager) CleanupIdleSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for id, wrapper := range sm.sessions {
		if now.Sub(wrapper.LastActive) > sm.timeout {
			delete(sm.sessions, id)
			cleaned++
		}
	}

	return cleaned
}
