package daemon

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDaemonManager_Lifecycle(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Start a short-lived process
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo daemon-test-ok"
	} else {
		cmd = "echo daemon-test-ok"
	}

	state, err := mgr.Start(cmd, "", nil)
	if err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}
	if state.ID == "" {
		t.Fatal("expected non-empty daemon ID")
	}

	// Give process a moment to exit
	time.Sleep(300 * time.Millisecond)

	updatedState, err := mgr.GetState(state.ID)
	if err != nil {
		t.Fatalf("failed to get daemon state: %v", err)
	}

	if updatedState.Status != "exited" {
		t.Logf("daemon status is %s (pid %d)", updatedState.Status, updatedState.Pid)
	}

	logs, err := mgr.ReadLogs(state.ID, 100)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	if !strings.Contains(logs, "daemon-test-ok") && !strings.Contains(logs, "exited with code") {
		t.Errorf("expected logs to contain output or exit message, got: %q", logs)
	}
}

func TestCheckProcessLiveness(t *testing.T) {
	// Negative PID should not be alive
	alive, _ := checkProcessLiveness(-1)
	if alive {
		t.Error("expected negative PID to not be alive")
	}

	// PID 0 should not be alive
	alive, _ = checkProcessLiveness(0)
	if alive {
		t.Error("expected PID 0 to not be alive")
	}
}
