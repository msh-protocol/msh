package execution

import (
	"testing"
	"time"
)

func TestSessionManager_GetOrCreateSession(t *testing.T) {
	sm := NewSessionManager(30 * time.Minute)

	// Test 1: Create a new session with empty ID
	s1, err := sm.GetOrCreateSession("", "")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if s1.ID == "" {
		t.Error("Session ID should be generated when empty ID is provided")
	}

	// Test 2: Retrieve the same session using the generated ID
	s2, err := sm.GetOrCreateSession(s1.ID, "")
	if err != nil {
		t.Fatalf("Failed to retrieve session: %v", err)
	}
	if s1 != s2 {
		t.Error("Expected to retrieve the exact same session instance")
	}

	// Test 3: Create a session with a custom ID
	customID := "test-session-123"
	s3, err := sm.GetOrCreateSession(customID, "")
	if err != nil {
		t.Fatalf("Failed to create custom session: %v", err)
	}
	if s3.ID != customID {
		t.Errorf("Expected session ID %q, got %q", customID, s3.ID)
	}
}

func TestSessionManager_CleanupIdleSessions(t *testing.T) {
	// Set a very short timeout for testing
	sm := NewSessionManager(50 * time.Millisecond)

	s1, _ := sm.GetOrCreateSession("keep-alive", "")
	_, _ = sm.GetOrCreateSession("expire-me", "")

	// Wait for half the timeout duration
	time.Sleep(30 * time.Millisecond)

	// Ping s1 to keep it alive
	_, _ = sm.GetOrCreateSession(s1.ID, "")

	// Wait enough time for the second session to expire
	time.Sleep(30 * time.Millisecond)

	cleaned := sm.CleanupIdleSessions()

	if cleaned != 1 {
		t.Errorf("Expected 1 session to be cleaned up, got %d", cleaned)
	}

	// Verify "keep-alive" is still there
	sm.mu.RLock()
	_, exists := sm.sessions["keep-alive"]
	sm.mu.RUnlock()
	if !exists {
		t.Error("Session 'keep-alive' should not have been cleaned up")
	}

	// Verify "expire-me" is gone
	sm.mu.RLock()
	_, exists = sm.sessions["expire-me"]
	sm.mu.RUnlock()
	if exists {
		t.Error("Session 'expire-me' should have been cleaned up")
	}
}
