package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/fleet"
	"github.com/msh-protocol/msh/pkg/fs"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// Server encapsulates the HTTP daemon logic for msh.
type Server struct {
	sessionManager *execution.SessionManager
	port           int
	host           string
	token          string
	fleet          string
}

// NewServer initializes a new msh HTTP server.
func NewServer(host string, port int, idleTimeout time.Duration, token string, fleet string) *Server {
	return &Server{
		sessionManager: execution.NewSessionManager(idleTimeout),
		port:           port,
		host:           host,
		token:          token,
		fleet:          fleet,
	}
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/execute", s.handleExecute)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stream/daemon", s.handleStreamDaemon)
	mux.HandleFunc("/stream/exec", s.handleStreamExec)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("msh server listening on http://%s\n", addr)

	// Start a background goroutine to clean up idle sessions
	go s.cleanupLoop()

	if s.fleet != "" {
		go s.connectToFleet()
	}

	return (&http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute, // commands can run up to default 30s + overhead
		IdleTimeout:  60 * time.Second,
	}).ListenAndServe()
}

func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.sessionManager.CleanupIdleSessions()
	}
}

func (s *Server) connectToFleet() {
	for {
		url := fmt.Sprintf("%s/register?token=%s", s.fleet, s.token)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			fmt.Printf("Failed to connect to fleet %s: %v. Retrying in 5s...\n", s.fleet, err)
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Printf("Connected to msh fleet hub at %s\n", s.fleet)

		// Send registration payload
		host, _ := os.Hostname()
		// We should import runtime for GOOS and GOARCH
		payload := map[string]string{
			"id":       s.token, // use token as unique ID for now
			"hostname": host,
		}
		conn.WriteJSON(payload)

		// Context for cancelling log tailing
		ctx, cancel := context.WithCancel(context.Background())

		// Read loop to detect disconnects and handle messages
		for {
			var msg fleet.FleetMsg
			if err := conn.ReadJSON(&msg); err != nil {
				fmt.Printf("Disconnected from fleet hub. Reconnecting in 5s...\n")
				break
			}

			switch msg.Type {
			case "stream_start":
				cancel() // cancel any existing stream
				ctx, cancel = context.WithCancel(context.Background())
				
				logPath := filepath.Join(".msh", "daemons", s.token+".log")
				outChan := make(chan []byte)

				go func(c context.Context, p string, ch chan []byte) {
					fs.TailFile(c, p, ch)
					close(ch)
				}(ctx, logPath, outChan)

				go func(c context.Context, ch chan []byte) {
					for {
						select {
						case <-c.Done():
							return
						case chunk, ok := <-ch:
							if !ok {
								return
							}
							conn.WriteJSON(fleet.FleetMsg{
								Type: "log",
								Data: string(chunk),
							})
						}
					}
				}(ctx, outChan)

			case "stream_stop":
				cancel()
			}
		}
		cancel() // ensure tailing stops if disconnected
		conn.Close()
		time.Sleep(5 * time.Second)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if s.token != "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+s.token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req, err := protocol.ParseExecRequest(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "command field is required", http.StatusBadRequest)
		return
	}

	// Retrieve or create session
	session, err := s.sessionManager.GetOrCreateSession(req.SessionID, "")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize session: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute command
	executor := execution.NewExecutor(session)
	resp := executor.Execute(*req)

	// Attach SessionID back to response
	resp.SessionID = session.ID

	// Send response
	respBytes, err := resp.ToJSON()
	if err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)

	// Asynchronously push to fleet history if connected
	if s.fleet != "" {
		go func(r protocol.ExecRequest, p protocol.ExecResponse) {
			payload := struct {
				Req  protocol.ExecRequest  `json:"request"`
				Resp protocol.ExecResponse `json:"response"`
			}{Req: r, Resp: p}

			payloadBytes, _ := json.Marshal(payload)
			
			// If fleet is ws://..., convert to http://...
			httpFleet := s.fleet
			if len(httpFleet) > 2 && httpFleet[:3] == "ws:" {
				httpFleet = "http:" + httpFleet[3:]
			} else if len(httpFleet) > 3 && httpFleet[:4] == "wss:" {
				httpFleet = "https:" + httpFleet[4:]
			}

			req, err := http.NewRequest("POST", httpFleet+"/api/history", bytes.NewBuffer(payloadBytes))
			if err == nil {
				if s.token != "" {
					req.Header.Set("Authorization", "Bearer "+s.token)
				}
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 5 * time.Second}
				client.Do(req)
			}
		}(*req, resp)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the local daemon
	},
}

func (s *Server) handleStreamDaemon(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query string
	if s.token != "" {
		token := r.URL.Query().Get("token")
		if token != s.token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	daemonID := r.URL.Query().Get("id")
	if daemonID == "" {
		http.Error(w, "id query parameter is required", http.StatusBadRequest)
		return
	}

	// Prevent path traversal
	if filepath.Base(daemonID) != daemonID {
		http.Error(w, "invalid daemon id", http.StatusBadRequest)
		return
	}

	// Upgrade the HTTP connection to a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Locate the daemon log file
	cwd, _ := os.Getwd()
	logPath := filepath.Join(cwd, ".msh", "daemons", daemonID+".log")

	// Create a channel for tailing
	outChan := make(chan []byte)

	// Context to stop the tailer when the client disconnects
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start tailing in the background
	go func() {
		err := fs.TailFile(ctx, logPath, outChan)
		if err != nil && ctx.Err() == nil {
			fmt.Printf("tail error: %v\n", err)
		}
		close(outChan)
	}()

	// Read loop to detect client disconnects
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				break
			}
		}
	}()

	// Write loop to push logs to the client
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-outChan:
			if !ok {
				return // channel closed
			}
			err = conn.WriteMessage(websocket.TextMessage, chunk)
			if err != nil {
				cancel()
				return
			}
		}
	}
}

// StreamMsg is a message exchanged over the /stream/exec WebSocket.
//
// Client → server:
//   {"type":"start","request":{...}}  begins a streaming run
//   {"type":"answer","data":"y"}      answers a pending prompt
//   {"type":"stop"}                   cancels the run
//
// Server → client:
//   {"type":"output","stream":"stdout","data":"..."}   raw output as it is read
//   {"type":"prompt","prompt":"...","awaiting":true}   run is waiting for input
//   {"type":"result","response":{...}}                 final ExecResponse
//   {"type":"error","error":"..."}                     protocol error
type StreamMsg struct {
	Type     string          `json:"type"`
	Stream   string          `json:"stream,omitempty"`
	Data     string          `json:"data,omitempty"`
	Prompt   string          `json:"prompt,omitempty"`
	Awaiting bool            `json:"awaiting,omitempty"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// wsSink relays execution event chunks onto a bounded channel destined for
// the WebSocket writer. Chunks are dropped when the client is too slow, so a
// stalled browser cannot pause command execution.
type wsSink struct {
	ch chan StreamMsg
}

func (w *wsSink) OnOutput(stream string, chunk []byte) {
	select {
	case w.ch <- StreamMsg{Type: "output", Stream: stream, Data: string(chunk)}:
	default:
	}
}

func (w *wsSink) OnPrompt(prompt string, answered bool) {
	select {
	case w.ch <- StreamMsg{Type: "prompt", Prompt: prompt, Awaiting: !answered}:
	default:
	}
}

// handleStreamExec is the live-execution endpoint: output is pushed in real
// time over WebSocket and prompts can be answered mid-flight via "answer"
// messages, instead of pre-supplying all answers up front.
func (s *Server) handleStreamExec(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query string (like /stream/daemon) or Bearer header.
	if s.token != "" {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
				tok = strings.TrimPrefix(a, "Bearer ")
			}
		}
		if tok != s.token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// First message must be a start request.
	var start StreamMsg
	if err := conn.ReadJSON(&start); err != nil {
		conn.WriteJSON(StreamMsg{Type: "error", Error: "expected start message: " + err.Error()})
		return
	}
	if start.Type != "start" || len(start.Request) == 0 {
		conn.WriteJSON(StreamMsg{Type: "error", Error: `first message must be {"type":"start","request":{...}}`})
		return
	}

	execReq, err := protocol.ParseExecRequest(start.Request)
	if err != nil {
		conn.WriteJSON(StreamMsg{Type: "error", Error: "invalid request: " + err.Error()})
		return
	}
	if execReq.Command == "" {
		conn.WriteJSON(StreamMsg{Type: "error", Error: "command field is required"})
		return
	}

	session, err := s.sessionManager.GetOrCreateSession(execReq.SessionID, "")
	if err != nil {
		conn.WriteJSON(StreamMsg{Type: "error", Error: "failed to initialize session: " + err.Error()})
		return
	}
	execReq.SessionID = session.ID

	// Apply the request timeout on top of the connection context, so both a
	// long-running command and a client disconnect kill the process.
	timeout := execReq.Timeout
	if timeout == 0 {
		timeout = protocol.DefaultTimeout
	}
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// Client answers arrive on this channel and feed the live AnswerProvider.
	answerCh := make(chan string, 8)

	// Writer drains sink events to the socket under a deadline so a stalled
	// client cannot hold the handler open forever.
	sinkCh := make(chan StreamMsg, 64)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for {
			select {
			case msg, ok := <-sinkCh:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(msg); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	sink := &wsSink{ch: sinkCh}

	// Read loop forwards answers and stop messages to the run.
	go func() {
		for {
			var msg StreamMsg
			if err := conn.ReadJSON(&msg); err != nil {
				cancel()
				return
			}
			switch msg.Type {
			case "answer":
				select {
				case answerCh <- msg.Data:
				case <-ctx.Done():
					return
				}
			case "stop":
				cancel()
				return
			}
		}
	}()

	answers := func() (string, bool) {
		select {
		case a := <-answerCh:
			return a, true
		case <-ctx.Done():
			return "", false
		}
	}

	executor := execution.NewExecutor(session)
	resp := executor.ExecuteStreaming(ctx, *execReq, sink, answers)
	resp.SessionID = session.ID

	respBytes, err := resp.ToJSON()
	if err != nil {
		respBytes, _ = json.Marshal(map[string]string{"error": "failed to serialize response"})
	}
	select {
	case sinkCh <- StreamMsg{Type: "result", Response: respBytes}:
	case <-ctx.Done():
	}

	close(sinkCh)
	<-writeDone
}
