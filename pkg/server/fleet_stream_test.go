package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msh-protocol/msh/pkg/fleet"
	"github.com/msh-protocol/msh/pkg/protocol"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// waitNode polls the hub until a daemon with the given ID registers.
func waitNode(t *testing.T, hubPort int, hubToken, wantID string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/nodes", hubPort), nil)
		req.Header.Set("Authorization", "Bearer "+hubToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		var nodes []fleet.NodeInfo
		decodeErr := json.NewDecoder(resp.Body).Decode(&nodes)
		resp.Body.Close()
		if decodeErr == nil {
			for _, n := range nodes {
				if n.ID == wantID {
					return
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("daemon %s never registered", wantID)
}

// TestFleetRemoteStreaming exercises the full relay path: a remote client
// connects to the fleet hub's /stream/exec, the hub relays the request to a
// registered daemon over its tunnel, the daemon streams output and prompts
// back, and the client's answers are forwarded to the live run.
func TestFleetRemoteStreaming(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("fleet streaming integration test relies on cmd batch prompts")
	}

	const token = "test-fleet-token"

	hub := fleet.NewServer("127.0.0.1", freePort(t), token)
	go hub.Start()

	daemon := NewServer("127.0.0.1", freePort(t), 5*time.Minute, token,
		fmt.Sprintf("ws://127.0.0.1:%d", hub.Port()))
	go daemon.Start()

	waitNode(t, hub.Port(), token, token)

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/stream/exec?token=%s&id=%s",
		hub.Port(), token, token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial remote stream: %v", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	rawReq, err := json.Marshal(protocol.ExecRequest{
		Command:        streamDoublePromptCmd(t),
		MaxOutputLines: 10,
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	if err := conn.WriteJSON(struct {
		Type    string          `json:"type"`
		Request json.RawMessage `json:"request"`
	}{Type: "start", Request: rawReq}); err != nil {
		t.Fatalf("failed to send start: %v", err)
	}

	answers := []string{"yes", "no"}
	answerIdx := 0
	var streamed strings.Builder
	var result protocol.ExecResponse

	for {
		var msg StreamMsg
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("failed to read remote message: %v", err)
		}
		switch msg.Type {
		case "output":
			streamed.WriteString(msg.Data)
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
			if result.Status != protocol.StatusSuccess {
				t.Fatalf("expected success, got %s (%s)", result.Status, result.Error)
			}
			if !strings.Contains(result.Stdout, "first=yes") ||
				!strings.Contains(result.Stdout, "second=no") {
				t.Fatalf("expected answered output in result, got %q", result.Stdout)
			}
			if !strings.Contains(streamed.String(), "first=yes") {
				t.Fatalf("expected streamed output to include answered echo, got %q", streamed.String())
			}
			return
		case "error":
			t.Fatalf("remote stream error: %s", msg.Error)
		default:
			t.Fatalf("unexpected message type: %s", msg.Type)
		}
	}
}