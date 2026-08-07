package fleet

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	fleetui "github.com/msh-protocol/msh/fleet-ui"
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
}

// NewServer initializes a new msh-fleet server.
func NewServer(host string, port int, token string) *Server {
	return &Server{
		host:  host,
		port:  port,
		token: token,
		nodes: make(map[string]*Node),
	}
}

// Start begins listening for daemon registrations and serves the dashboard.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/api/nodes", s.handleListNodes)
	mux.HandleFunc("/stream/daemon", s.handleStreamDaemon)

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
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}).ListenAndServe()
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && r.Header.Get("Authorization") != "Bearer "+s.token {
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
	if s.token != "" && r.URL.Query().Get("token") != s.token {
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
			if msg.Type == "log" {
				select {
				case node.StreamChan <- msg.Data:
				default:
					// drop if channel is full
				}
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

func (s *Server) handleStreamDaemon(w http.ResponseWriter, r *http.Request) {
	// Wait, the UI connects to /stream/daemon. We need no auth for UI stream since UI doesn't send Bearer easily in WS (it uses query param)
	// But actually, UI CAN send query param token! Let's check it.
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

	// Tell agent to start streaming
	node.conn.WriteJSON(FleetMsg{Type: "stream_start"})

	// Setup cleanup to tell agent to stop
	defer node.conn.WriteJSON(FleetMsg{Type: "stream_stop"})

	// Read from UI to detect UI disconnect
	go func() {
		for {
			if _, _, err := uiConn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// Relay logs from agent to UI
	for chunk := range node.StreamChan {
		if err := uiConn.WriteMessage(websocket.TextMessage, []byte(chunk)); err != nil {
			return // UI disconnected
		}
	}
}
