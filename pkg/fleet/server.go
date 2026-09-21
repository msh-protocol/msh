package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	fleetui "github.com/msh-protocol/msh/fleet-ui"
	"github.com/msh-protocol/msh/pkg/db"
	"github.com/msh-protocol/msh/pkg/execution"
	mshfs "github.com/msh-protocol/msh/pkg/fs"
	"github.com/msh-protocol/msh/pkg/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the fleet registry
	},
}

// Server acts as the central hub for msh daemons.
type Server struct {
	host  string
	port  int
	token string

	mu    sync.RWMutex
	nodes map[string]*Node
	db    *db.DB

	ExecutionQueue chan protocol.ExecRequest

	// streamMu guards the relayed-exec stream registry: each entry maps a
	// daemon-side stream ID to a remote client websocket.
	streamMu sync.Mutex
	streams  map[string]*streamConn
}

// NewServer initializes a new msh-fleet server.
func NewServer(host string, port int, token string) *Server {
	database, err := db.InitDB()
	if err != nil {
		fmt.Printf("Warning: Failed to initialize SQLite database: %v\n", err)
	}

	s := &Server{
		host:           host,
		port:           port,
		token:          token,
		nodes:          make(map[string]*Node),
		db:             database,
		ExecutionQueue: make(chan protocol.ExecRequest, 1000),
		streams:        make(map[string]*streamConn),
	}
	go s.autoScaler()
	return s
}

// Port returns the configured listen port.
func (s *Server) Port() int {
	return s.port
}

// Start begins listening for daemon registrations and serves the dashboard.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/api/nodes", s.handleListNodes)
	mux.HandleFunc("/api/nodes/ping", s.handlePingNode)
	mux.HandleFunc("/api/execute", s.handleExecute)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/diff", s.handleDiff)
	mux.HandleFunc("/api/rollback", s.handleRollback)
	mux.HandleFunc("/api/verify", s.handleVerify)
	mux.HandleFunc("/stream/daemon", s.handleStreamDaemon)
	mux.HandleFunc("/stream/exec", s.handleStreamExec)

	distFS, err := fs.Sub(fleetui.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded UI: %v", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	}))

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("msh fleet hub listening on http://%s\n", addr)

	return (&http.Server{
		Addr:    addr,
		Handler: mux,
	}).ListenAndServe()
}

// NodeDetail represents node telemetry returned to the UI.
type NodeDetail struct {
	ID          string `json:"id"`
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	ConnectedAt string `json:"connected_at,omitempty"`
	UptimeSec   int64  `json:"uptime_sec,omitempty"`
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var nodes []NodeDetail
	for _, n := range s.nodes {
		nodes = append(nodes, NodeDetail{
			ID:          n.ID,
			Hostname:    n.Hostname,
			OS:          n.OS,
			Arch:        n.Arch,
			ConnectedAt: n.Connected.Format(time.RFC3339),
			UptimeSec:   int64(now.Sub(n.Connected).Seconds()),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (s *Server) handlePingNode(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	nodeID := r.URL.Query().Get("id")
	if nodeID == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	node, exists := s.nodes[nodeID]
	s.mu.RUnlock()

	if !exists || node == nil || node.conn == nil {
		http.Error(w, "Node not found or disconnected", http.StatusNotFound)
		return
	}

	start := time.Now()
	err := node.conn.WriteControl(websocket.PingMessage, []byte("msh-ping"), time.Now().Add(2*time.Second))
	latency := time.Since(start)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     nodeID,
			"online": false,
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         nodeID,
		"online":     true,
		"latency_ms": latency.Milliseconds(),
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.URL.Query().Get("token"), s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Fleet registry WebSocket upgrade failed: %v\n", err)
		return
	}

	// Wait for the node to send its NodeInfo
	var info NodeInfo
	if err := conn.ReadJSON(&info); err != nil {
		fmt.Printf("Failed to read NodeInfo during registration: %v\n", err)
		conn.Close()
		return
	}

	node := NewNode(info, conn)
	s.registerNode(node)

	go node.writePump()

	// Read loop to detect disconnects and handle incoming messages
	go func() {
		defer s.unregisterNode(node.ID)
		for {
			var msg FleetMsg
			if err := conn.ReadJSON(&msg); err != nil {
				break
			}
			switch msg.Type {
			case "log":
				select {
				case node.StreamChan <- msg.Data:
				default:
					// drop if channel is full
				}
			case "exec_output", "exec_prompt", "exec_result", "exec_error":
				s.relayExec(msg)
			}
		}
	}()
}

func (s *Server) registerNode(n *Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// If a node with the same ID already exists, close its connection first
	if existing, ok := s.nodes[n.ID]; ok {
		existing.conn.Close()
	}
	
	s.nodes[n.ID] = n
	fmt.Printf("Node registered: %s (%s - %s)\n", n.ID, n.Hostname, n.OS)
}

func (s *Server) unregisterNode(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.nodes[id]; ok {
		n.conn.Close()
		delete(s.nodes, id)
		fmt.Printf("Node unregistered: %s\n", id)
	}
}

// streamConn is the hub-side half of a relayed exec stream. send feeds a
// single writer pump; terminal is closed once the stream finishes (or the
// client disconnects), which also closes send so the pump drains and exits.
type streamConn struct {
	req      protocol.ExecRequest
	send     chan []byte
	terminal chan struct{}
	once     sync.Once
}

func newStreamConn(req protocol.ExecRequest) *streamConn {
	return &streamConn{
		req:      req,
		send:     make(chan []byte, 128),
		terminal: make(chan struct{}),
	}
}

func (sc *streamConn) close() {
	sc.once.Do(func() {
		close(sc.terminal)
		close(sc.send)
	})
}

func (s *Server) registerStream(id string, sc *streamConn) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	s.streams[id] = sc
}

func (s *Server) unregisterStream(id string) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	delete(s.streams, id)
}

// relayExec forwards a daemon→hub exec event to the remote client that owns
// the stream. The message Data is already in the client-facing wire format,
// so it is passed through verbatim. Terminal events finish the stream.
func (s *Server) relayExec(msg FleetMsg) {
	s.streamMu.Lock()
	sc := s.streams[msg.ID]
	s.streamMu.Unlock()
	if sc == nil {
		return
	}
	if msg.Type == "exec_result" || msg.Type == "exec_error" {
		select {
		case sc.send <- []byte(msg.Data):
		default:
		}
		if s.db != nil {
			switch msg.Type {
			case "exec_result":
				var ev ExecEvent
				if err := json.Unmarshal([]byte(msg.Data), &ev); err == nil && len(ev.Response) > 0 {
					var resp protocol.ExecResponse
					if err := json.Unmarshal(ev.Response, &resp); err == nil {
						_ = s.db.SaveExecution(sc.req, resp)
					}
				}
			case "exec_error":
				var ev ExecEvent
				_ = json.Unmarshal([]byte(msg.Data), &ev)
				errMsg := ev.Error
				if errMsg == "" {
					errMsg = "streaming execution error"
				}
				resp := protocol.ExecResponse{
					Status:   protocol.StatusError,
					Stderr:   errMsg,
					ExitCode: -1,
				}
				_ = s.db.SaveExecution(sc.req, resp)
			}
		}
		sc.close()
		return
	}
	select {
	case sc.send <- []byte(msg.Data):
	default:
		// drop slow consumers
	}
}

// hubRelayMsg is the client-facing wire frame on the hub's /stream/exec
// endpoint. It intentionally mirrors the local daemon /stream/exec protocol.
type hubRelayMsg struct {
	Type    string          `json:"type"`
	Data    string          `json:"data,omitempty"`
	Request json.RawMessage `json:"request,omitempty"`
}

// handleStreamExec lets a remote client stream a command to a registered
// daemon. The client opens a WebSocket, sends a start frame carrying the
// ExecRequest, and then receives output/prompt/result events while sending
// answer/stop frames. The hub relays everything over the target daemon's
// tunnel.
func (s *Server) handleStreamExec(w http.ResponseWriter, r *http.Request) {
	if s.token != "" {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
				tok = strings.TrimPrefix(a, "Bearer ")
			}
		}
		if !protocol.SecureCompare(tok, s.token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	nodeID := r.URL.Query().Get("id")
	s.mu.RLock()
	var node *Node
	if nodeID != "" {
		node = s.nodes[nodeID]
	} else {
		for _, n := range s.nodes {
			node = n
			break
		}
	}
	s.mu.RUnlock()
	if node == nil {
		http.Error(w, "no daemon available", http.StatusServiceUnavailable)
		return
	}

	client, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Exec stream WebSocket upgrade failed: %v\n", err)
		return
	}

	// First frame must be a start that carries the ExecRequest.
	var start hubRelayMsg
	if err := client.ReadJSON(&start); err != nil || start.Type != "start" || len(start.Request) == 0 {
		client.WriteJSON(hubRelayMsg{Type: "error", Data: "expected a start frame with a request"})
		client.Close()
		return
	}

	var execReq protocol.ExecRequest
	if parsed, err := protocol.ParseExecRequest(start.Request); err == nil {
		execReq = *parsed
	}

	sc := newStreamConn(execReq)
	streamID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	s.registerStream(streamID, sc)
	defer s.unregisterStream(streamID)

	if err := node.SendMessage(FleetMsg{Type: "exec_start", ID: streamID, Request: start.Request}); err != nil {
		select {
		case sc.send <- mustMarshal(hubRelayMsg{Type: "error", Data: "daemon unavailable: " + err.Error()}):
		default:
		}
		sc.close()
		client.Close()
		return
	}

	// Writer pump: drains relayed events to the client.
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for b := range sc.send {
			client.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		}
	}()

	// Forward client frames (answer/stop) to the daemon. Closing the client
	// stops the remote run.
	go func() {
		for {
			var m hubRelayMsg
			if err := client.ReadJSON(&m); err != nil {
				node.SendMessage(FleetMsg{Type: "exec_stop", ID: streamID})
				sc.close()
				return
			}
			switch m.Type {
			case "answer":
				node.SendMessage(FleetMsg{Type: "exec_answer", ID: streamID, Data: m.Data})
			case "stop":
				node.SendMessage(FleetMsg{Type: "exec_stop", ID: streamID})
				sc.close()
				return
			}
		}
	}()

	// Wait for the daemon to finish the stream.
	<-sc.terminal
	<-writerDone
	client.Close()
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","data":"marshal failed"}`)
	}
	return b
}

func (s *Server) handleStreamDaemon(w http.ResponseWriter, r *http.Request) {
	// Wait, the UI connects to /stream/daemon. We need no auth for UI stream since UI doesn't send Bearer easily in WS (it uses query param)
	// But actually, UI CAN send query param token! Let's check it.
	if s.token != "" {
		token := r.URL.Query().Get("token")
		if !protocol.SecureCompare(token, s.token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	daemonID := r.URL.Query().Get("id")
	if daemonID == "" {
		http.Error(w, "id query parameter is required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	node, ok := s.nodes[daemonID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "daemon not found", http.StatusNotFound)
		return
	}

	uiConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("UI WebSocket upgrade failed: %v\n", err)
		return
	}
	defer uiConn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Tell agent to start streaming
	node.SendMessage(FleetMsg{Type: "stream_start"})

	// Setup cleanup to tell agent to stop
	defer node.SendMessage(FleetMsg{Type: "stream_stop"})

	// Read from UI to detect UI disconnect
	go func() {
		for {
			if _, _, err := uiConn.ReadMessage(); err != nil {
				cancel()
				break
			}
		}
	}()

	// Relay logs from agent to UI
	for {
		select {
		case <-ctx.Done():
			return
		case chunk := <-node.StreamChan:
			if err := uiConn.WriteMessage(websocket.TextMessage, []byte(chunk)); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
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

	select {
	case s.ExecutionQueue <- *req:
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"queued"}`))
	default:
		http.Error(w, "Execution queue full", http.StatusServiceUnavailable)
	}
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Add CORS headers for the React dashboard
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodDelete {
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			id, err := strconv.Atoi(idStr)
			if err != nil || id <= 0 {
				http.Error(w, "Invalid ID", http.StatusBadRequest)
				return
			}
			if err := s.db.DeleteExecution(id); err != nil {
				http.Error(w, fmt.Sprintf("Failed to delete record: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Deleted execution #%d", id),
			})
			return
		}

		if r.URL.Query().Get("all") == "true" {
			if err := s.db.ClearExecutions(); err != nil {
				http.Error(w, fmt.Sprintf("Failed to clear history: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Cleared all execution records",
			})
			return
		}

		http.Error(w, "Missing id or all query parameter", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		// Agent reporting execution result
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var payload struct {
			Req  protocol.ExecRequest  `json:"request"`
			Resp protocol.ExecResponse `json:"response"`
		}

		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
			return
		}

		if err := s.db.SaveExecution(payload.Req, payload.Resp); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	if r.Method == http.MethodGet {
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err == nil && n > 0 {
				limit = n
			}
		}
		if limit > 200 {
			limit = 200
		}

		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			n, err := strconv.Atoi(v)
			if err == nil && n > 0 {
				offset = n
			}
		}

		records, err := s.db.GetExecutions(limit, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch history: %v", err), http.StatusInternalServerError)
			return
		}

		total, err := s.db.Count()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch history count: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		json.NewEncoder(w).Encode(records)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	activeDaemons := len(s.nodes)
	s.mu.RUnlock()

	dbMetrics, err := s.db.GetMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to compute metrics: %v", err), http.StatusInternalServerError)
		return
	}

	avgLat := math.Round(dbMetrics.AvgLatencyMs*10) / 10
	succRate := math.Round(dbMetrics.SuccessRate*10) / 10

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_executions":  dbMetrics.TotalExecutions,
		"active_daemons":    activeDaemons,
		"avg_latency_ms":    avgLat,
		"success_count":     dbMetrics.SuccessCount,
		"error_count":       dbMetrics.ErrorCount,
		"success_rate":      succRate,
		"total_duration_ms": dbMetrics.TotalDurationMs,
		"top_commands":      dbMetrics.TopCommands,
		"recent_history":    dbMetrics.RecentHistory,
		"error_breakdown":   dbMetrics.ErrorBreakdown,
	})
}

func (s *Server) autoScaler() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		queueDepth := len(s.ExecutionQueue)
		s.mu.RLock()
		nodeCount := len(s.nodes)
		s.mu.RUnlock()

		// Simple auto-scaling logic: if we have items in the queue and 0 nodes,
		// or queue depth is much larger than node count, spawn a new daemon.
		if queueDepth > 0 && (nodeCount == 0 || queueDepth > nodeCount*5) {
			// Prevent unbounded scaling for this prototype (max 5 nodes)
			if nodeCount < 5 {
				fmt.Printf("[Auto-Scaler] Queue depth: %d, Nodes: %d. Spawning new msh daemon...\n", queueDepth, nodeCount)
				
				// Find msh binary
				mshPath, err := exec.LookPath("msh")
				if err == nil {
					// Spawn a new background agent connecting back to us
					cmd := exec.Command(mshPath, "serve", "--fleet", fmt.Sprintf("ws://127.0.0.1:%d", s.port), "--token", s.token)
					err := cmd.Start()
					if err != nil {
						fmt.Printf("[Auto-Scaler] Failed to spawn daemon: %v\n", err)
					} else {
						// Detach from the child process so it survives
						go func(c *exec.Cmd) {
							c.Wait()
							fmt.Printf("[Auto-Scaler] Daemon exited.\n")
						}(cmd)
					}
				}
			}
		}
	}
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		http.Error(w, "file query parameter is required", http.StatusBadRequest)
		return
	}

	cwd := r.URL.Query().Get("cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	diffs := mshfs.GenerateDiffs(cwd, []string{filePath})
	diff := ""
	if diffs != nil {
		diff = diffs[filePath]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"file": filePath,
		"diff": diff,
	})
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ID   int    `json:"id"`
		File string `json:"file,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID <= 0 {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	records, err := s.db.GetExecutions(200, 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query history: %v", err), http.StatusInternalServerError)
		return
	}

	var targetRecord *db.ExecutionRecord
	for i := range records {
		if records[i].ID == payload.ID {
			targetRecord = &records[i]
			break
		}
	}
	if targetRecord == nil {
		http.Error(w, "Execution record not found", http.StatusNotFound)
		return
	}

	var resp protocol.ExecResponse
	if err := json.Unmarshal([]byte(targetRecord.RespJSON), &resp); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse response JSON: %v", err), http.StatusInternalServerError)
		return
	}

	cwd := resp.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	w.Header().Set("Content-Type", "application/json")

	// Surgical single-file rollback if file is specified
	if payload.File != "" {
		targetFile := payload.File
		diff := resp.FileDiffs[targetFile]
		if diff == "" {
			diff = resp.FileDiffs[filepath.ToSlash(targetFile)]
		}
		if diff == "" {
			diff = resp.FileDiffs[filepath.FromSlash(targetFile)]
		}

		if err := mshfs.RollbackSingleFile(cwd, targetFile, diff); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":        true,
			"reverted_files": []string{targetFile},
			"message":        fmt.Sprintf("Successfully rolled back %s", targetFile),
		})
		return
	}

	// Full execution rollback
	reverted, err := mshfs.RollbackExecution(cwd, resp.FilesChanged, resp.FileDiffs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"reverted_files": reverted,
		"message":        fmt.Sprintf("Successfully rolled back %d file(s)", len(reverted)),
	})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID <= 0 {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	records, err := s.db.GetExecutions(200, 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query history: %v", err), http.StatusInternalServerError)
		return
	}

	var targetRecord *db.ExecutionRecord
	for i := range records {
		if records[i].ID == payload.ID {
			targetRecord = &records[i]
			break
		}
	}
	if targetRecord == nil {
		http.Error(w, "Execution record not found", http.StatusNotFound)
		return
	}

	var req protocol.ExecRequest
	if err := json.Unmarshal([]byte(targetRecord.ReqJSON), &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse request JSON: %v", err), http.StatusInternalServerError)
		return
	}

	var resp protocol.ExecResponse
	if err := json.Unmarshal([]byte(targetRecord.RespJSON), &resp); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse response JSON: %v", err), http.StatusInternalServerError)
		return
	}

	result, err := execution.VerifyExecution(req, resp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Verification failed: %v", err), http.StatusInternalServerError)
		return
	}

	result.RunID = payload.ID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
