package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/msh-protocol/msh/pkg/protocol"
)

func TestServer_Health(t *testing.T) {
	srv := NewServer("127.0.0.1", 8080, 5*time.Minute, "")

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(srv.handleHealth)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"ok"}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestServer_Execute(t *testing.T) {
	srv := NewServer("127.0.0.1", 8080, 5*time.Minute, "")

	execReq := protocol.ExecRequest{
		Command: "echo hello server",
	}
	body, _ := json.Marshal(execReq)

	req, err := http.NewRequest("POST", "/execute", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(srv.handleExecute)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp protocol.ExecResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if resp.Status != protocol.StatusSuccess {
		t.Errorf("Expected status success, got %v", resp.Status)
	}
	if resp.Stdout != "hello server" {
		t.Errorf("Expected stdout 'hello server', got %q", resp.Stdout)
	}
	if resp.SessionID == "" {
		t.Error("Expected server to assign a SessionID")
	}

	// Test state persistence via SessionID
	// We run `cd ..` then `pwd` on Unix, or `cd ..` and `cd` on Windows.
	// Since we are running on Windows, `cd` prints current directory.
	// Let's just pass `cd ..` and ensure SessionID works.
	execReq2 := protocol.ExecRequest{
		Command:   "cd ..",
		SessionID: resp.SessionID,
	}
	body2, _ := json.Marshal(execReq2)
	req2, _ := http.NewRequest("POST", "/execute", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed: %v", rr2.Code)
	}
	var resp2 protocol.ExecResponse
	json.Unmarshal(rr2.Body.Bytes(), &resp2)

	if resp2.SessionID != resp.SessionID {
		t.Errorf("Expected SessionID to remain the same, got %q", resp2.SessionID)
	}
	if resp2.Cwd == resp.Cwd {
		t.Error("Expected Cwd to change after 'cd ..'")
	}
}
