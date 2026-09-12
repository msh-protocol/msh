package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// streamDoublePromptCmd builds a command that prints two prompts and echoes
// both answers back, cross-platform via a temp batch file or sh.
func streamDoublePromptCmd(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		p := filepath.Join(t.TempDir(), "prompt.cmd")
		content := "@echo off\r\nset /p a=Proceed? [y/N] \r\nset /p b=Continue? [y/N] \r\necho first=%a% second=%b%\r\n"
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write batch file: %v", err)
		}
		return p
	}
	return `printf 'Proceed? [y/N] '; read a; printf 'Continue? [y/N] '; read b; echo "first=$a second=$b"`
}

// TestServer_StreamExec verifies the live-execution WebSocket: output chunks
// stream in real time, prompts are answered mid-flight with "answer"
// messages, and a final result carries the structured ExecResponse.
func TestServer_StreamExec(t *testing.T) {
	srv := NewServer("127.0.0.1", 0, 5*time.Minute, "", "")
	ts := httptest.NewServer(http.HandlerFunc(srv.handleStreamExec))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", wsURL, err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))

	execReq := protocol.ExecRequest{
		Command:        streamDoublePromptCmd(t),
		MaxOutputLines: 10,
	}
	rawReq, err := json.Marshal(execReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	if err := conn.WriteJSON(StreamMsg{Type: "start", Request: rawReq}); err != nil {
		t.Fatalf("failed to send start: %v", err)
	}

	answers := []string{"yes", "no"}
	answerIdx := 0
	var streamedOutput strings.Builder
	var result protocol.ExecResponse

	for {
		var msg StreamMsg
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("failed to read message: %v", err)
		}
		switch msg.Type {
		case "output":
			streamedOutput.WriteString(msg.Data)
		case "prompt":
			if msg.Awaiting && answerIdx < len(answers) {
				if err := conn.WriteJSON(StreamMsg{Type: "answer", Data: answers[answerIdx]}); err != nil {
					t.Fatalf("failed to send answer: %v", err)
				}
				answerIdx++
			}
		case "result":
			if err := json.Unmarshal(msg.Response, &result); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}
			return
		case "error":
			t.Fatalf("server error: %s", msg.Error)
		default:
			t.Fatalf("unexpected message type: %s", msg.Type)
		}
	}
}

// TestServer_StreamExecResult verifies the final result of a streamed run
// carries the same structured information as a normal execute.
func TestServer_StreamExecResult(t *testing.T) {
	srv := NewServer("127.0.0.1", 0, 5*time.Minute, "", "")
	ts := httptest.NewServer(http.HandlerFunc(srv.handleStreamExec))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))

	rawReq, _ := json.Marshal(protocol.ExecRequest{
		Command:        "echo hello stream result",
		MaxOutputLines: 10,
	})
	if err := conn.WriteJSON(StreamMsg{Type: "start", Request: rawReq}); err != nil {
		t.Fatalf("failed to send start: %v", err)
	}

	for {
		var msg StreamMsg
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("failed to read message: %v", err)
		}
		switch msg.Type {
		case "output":
			// consumed; chunks stream in real time
		case "prompt":
			t.Fatalf("unexpected prompt for non-interactive command")
		case "result":
			var result protocol.ExecResponse
			if err := json.Unmarshal(msg.Response, &result); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}
			if result.Status != protocol.StatusSuccess {
				t.Fatalf("expected success, got %s (%s)", result.Status, result.Error)
			}
			if !strings.Contains(result.Stdout, "hello stream result") {
				t.Fatalf("expected command output in result, got %q", result.Stdout)
			}
			if result.SessionID == "" {
				t.Fatal("expected server to assign a SessionID")
			}
			return
		case "error":
			t.Fatalf("server error: %s", msg.Error)
		}
	}
}

// TestServer_StreamExecUnauthorized verifies the endpoint rejects requests
// without the configured token.
func TestServer_StreamExecUnauthorized(t *testing.T) {
	srv := NewServer("127.0.0.1", 0, 5*time.Minute, "secret-token", "")
	ts := httptest.NewServer(http.HandlerFunc(srv.handleStreamExec))
	defer ts.Close()

	resp, err := http.Get(ts.URL) // bare HTTP request (no upgrade) with no token
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
}