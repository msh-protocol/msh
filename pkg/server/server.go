package server

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// Server encapsulates the HTTP daemon logic for msh.
type Server struct {
	sessionManager *execution.SessionManager
	port           int
	host           string
	token          string
}

// NewServer initializes a new msh HTTP server.
func NewServer(host string, port int, idleTimeout time.Duration, token string) *Server {
	return &Server{
		sessionManager: execution.NewSessionManager(idleTimeout),
		port:           port,
		host:           host,
		token:          token,
	}
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/execute", s.handleExecute)
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("msh server listening on http://%s\n", addr)

	// Start a background goroutine to clean up idle sessions
	go s.cleanupLoop()

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
