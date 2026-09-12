package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/msh-protocol/msh/pkg/protocol"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.json")
	if err := os.WriteFile(path, []byte("[]"), 0644); err != nil {
		t.Fatalf("write test db: %v", err)
	}
	return &DB{path: path}
}

func saveExec(t *testing.T, d *DB, sessionID, command string) {
	t.Helper()
	req := protocol.ExecRequest{SessionID: sessionID, Command: command}
	resp := protocol.ExecResponse{Status: protocol.StatusSuccess, ExitCode: 0, DurationMs: 12}
	if err := d.SaveExecution(req, resp); err != nil {
		t.Fatalf("SaveExecution: %v", err)
	}
}

func TestGetExecutionsPagination(t *testing.T) {
	d := newTestDB(t)
	for i := 0; i < 10; i++ {
		saveExec(t, d, "sess", "cmd")
	}

	total, err := d.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != 10 {
		t.Fatalf("Count() = %d, want 10", total)
	}

	// Newest first: first save has ID 10 at index 0, last save ID 1 at index 9.
	got, err := d.GetExecutions(3, 2)
	if err != nil {
		t.Fatalf("GetExecutions: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0].ID != 8 || got[1].ID != 7 || got[2].ID != 6 {
		t.Fatalf("got IDs %d,%d,%d, want 8,7,6", got[0].ID, got[1].ID, got[2].ID)
	}

	// Records carry the raw request/response JSON for replay.
	if got[0].ReqJSON == "" || got[0].RespJSON == "" {
		t.Fatalf("expected ReqJSON/RespJSON to be populated for replay")
	}
}

func TestGetExecutionsClampsBounds(t *testing.T) {
	d := newTestDB(t)
	for i := 0; i < 5; i++ {
		saveExec(t, d, "sess", "cmd")
	}

	// Offset beyond the record count returns an empty slice.
	got, err := d.GetExecutions(10, 100)
	if err != nil {
		t.Fatalf("GetExecutions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}

	// Limit beyond the remaining records clamps to what exists.
	got, err = d.GetExecutions(100, 3)
	if err != nil {
		t.Fatalf("GetExecutions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	// Empty database.
	empty := newTestDB(t)
	got, err = empty.GetExecutions(10, 0)
	if err != nil {
		t.Fatalf("GetExecutions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
	count, err := empty.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count() = %d, want 0", count)
	}
}