package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExecResponseToJSON(t *testing.T) {
	resp := &ExecResponse{
		Status:   StatusSuccess,
		ExitCode: 0,
		Cwd:      "/workspace",
		Stdout:   "hello world",
		Stderr:   "",
	}

	data, err := resp.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if parsed["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", parsed["status"])
	}

	if parsed["stdout"] != "hello world" {
		t.Errorf("Expected stdout 'hello world', got '%v'", parsed["stdout"])
	}
}

func TestExecResponseToPrettyJSON(t *testing.T) {
	resp := &ExecResponse{
		Status:   StatusError,
		ExitCode: 1,
		Cwd:      "/workspace",
		Stderr:   "file not found",
	}

	data, err := resp.ToPrettyJSON()
	if err != nil {
		t.Fatalf("ToPrettyJSON() failed: %v", err)
	}

	// Pretty JSON should contain newlines and indentation
	output := string(data)
	if len(output) == 0 {
		t.Fatal("ToPrettyJSON() returned empty output")
	}

	// Verify indentation exists
	if !contains(output, "\n") {
		t.Error("Pretty JSON should contain newlines")
	}
}

func TestParseExecRequest(t *testing.T) {
	input := `{
		"command": "npm run build",
		"cwd": "/workspace/app",
		"max_output_lines": 100,
		"detect_files": true
	}`

	req, err := ParseExecRequest([]byte(input))
	if err != nil {
		t.Fatalf("ParseExecRequest() failed: %v", err)
	}

	if req.Command != "npm run build" {
		t.Errorf("Expected command 'npm run build', got '%s'", req.Command)
	}

	if req.Cwd != "/workspace/app" {
		t.Errorf("Expected cwd '/workspace/app', got '%s'", req.Cwd)
	}

	if req.MaxOutputLines != 100 {
		t.Errorf("Expected max_output_lines 100, got %d", req.MaxOutputLines)
	}

	if !req.DetectFiles {
		t.Error("Expected detect_files to be true")
	}
}

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusSuccess, "success"},
		{StatusError, "error"},
		{StatusTimeout, "timeout"},
		{StatusBlocked, "blocked"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("Status %v: expected '%s', got '%s'", tt.status, tt.want, string(tt.status))
		}
	}
}

func TestDefaults(t *testing.T) {
	if DefaultMaxOutputLines != 200 {
		t.Errorf("Expected DefaultMaxOutputLines 200, got %d", DefaultMaxOutputLines)
	}

	if DefaultTimeout != 30*time.Second {
		t.Errorf("Expected DefaultTimeout 30s, got %v", DefaultTimeout)
	}

	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
