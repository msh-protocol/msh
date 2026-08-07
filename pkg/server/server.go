package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msh-protocol/msh/pkg/execution"
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

		// Wait for disconnect
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				fmt.Printf("Disconnected from fleet hub. Reconnecting in 5s...\n")
				break
			}
		}
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
