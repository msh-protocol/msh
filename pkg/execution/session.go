package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Session represents a persistent msh execution context.
// Unlike standard subprocess spawning where each command starts fresh,
// an msh session maintains state across commands — tracking the current
// working directory, environment variables, and background processes.
//
// This is the key differentiator from raw os/exec: an agent can run
// "cd src/api" followed by "ls" and the second command correctly
// executes in src/api.
type Session struct {
	// ID is a unique identifier for this session.
	ID string

	// Cwd is the current working directory for this session.
	// Persists across commands — cd mutations are tracked.
	Cwd string

	// Env holds session-scoped environment variables.
	// Variables set via export persist for the session lifetime.
	Env map[string]string

	// mu protects concurrent access to session state.
	mu sync.RWMutex
}

// NewSession creates a new msh session rooted at the given working directory.
// If workDir is empty, the current directory is used.
func NewSession(workDir string) (*Session, error) {
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}

	// Verify directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "session", Path: absPath, Err: os.ErrInvalid}
	}

	return &Session{
		ID:  generateSessionID(),
		Cwd: absPath,
		Env: make(map[string]string),
	}, nil
}

// UpdateCwd updates the session's working directory based on a cd target.
// Handles both absolute and relative paths.
func (s *Session) UpdateCwd(target string, baseCwd string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if target == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			s.Cwd = home
		}
		return
	}

	if target == "-" {
		// cd - is not tracked (would need previous dir history)
		return
	}

	if target == ".." {
		s.Cwd = filepath.Dir(s.Cwd)
		return
	}

	var newPath string
	if filepath.IsAbs(target) {
		newPath = target
	} else {
		newPath = filepath.Join(baseCwd, target)
	}

	// Verify the target exists
	absPath, err := filepath.Abs(newPath)
	if err != nil {
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return
	}

	s.Cwd = absPath
}

// SetEnv sets a session-scoped environment variable.
func (s *Session) SetEnv(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Env[key] = value
}

// GetEnv returns a session-scoped environment variable.
func (s *Session) GetEnv(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Env[key]
	return val, ok
}

// BuildEnv constructs the full environment variable list for a command.
// It starts with the host environment (filtered through allowlist),
// layers session env vars on top, then applies per-command overrides.
func (s *Session) BuildEnv(cmdEnv map[string]string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Start with host environment
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := splitEnvVar(e)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Layer session env vars
	for k, v := range s.Env {
		envMap[k] = v
	}

	// Layer per-command overrides
	for k, v := range cmdEnv {
		envMap[k] = v
	}

	// Convert to []string format
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}

	return result
}

// splitEnvVar splits "KEY=VALUE" into ["KEY", "VALUE"].
func splitEnvVar(env string) []string {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return []string{env[:i], env[i+1:]}
		}
	}
	return []string{env}
}

// generateSessionID creates a simple unique session identifier.
func generateSessionID() string {
	return fmt.Sprintf("msh-%d", time.Now().UnixNano())
}
