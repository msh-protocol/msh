package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	fleetui "github.com/msh-protocol/msh/fleet-ui"
	"github.com/msh-protocol/msh/pkg/protocol"
	"github.com/msh-protocol/msh/pkg/db"
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
	mux.HandleFunc("/api/execute", s.handleExecute)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
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

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && !protocol.SecureCompare(r.Header.Get("Authorization"), "Bearer "+s.token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var nodes []NodeInfo
	for _, n := range s.nodes {
		nodes = append(nodes, NodeInfo{
			ID:       n.ID,
			Hostname: n.Hostname,
			OS:       n.OS,
			Arch:     n.Arch,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
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
	send     chan []byte
	terminal chan struct{}
	once     sync.Once
}

func newStreamConn() *streamConn {
	return &streamConn{
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

	sc := newStreamConn()
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
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
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
