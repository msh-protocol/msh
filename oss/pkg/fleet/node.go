package fleet

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// NodeInfo is the payload a daemon sends when registering.
type NodeInfo struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// FleetMsg is a message exchanged over the hub↔daemon tunnel.
// Type carriers: "log" (daemon→hub), registration, and the relayed-exec
// family: exec_start / exec_answer / exec_stop (hub→daemon) and
// exec_output / exec_prompt / exec_result / exec_error (daemon→hub).
// For relayed events, Data holds the serialized ExecEvent JSON sent back to
// the remote client; the hub forwards it verbatim.
type FleetMsg struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Data    string          `json:"data"`
	Request json.RawMessage `json:"request,omitempty"`
}

// ExecEvent mirrors the streaming protocol a /stream/exec client receives,
// transported over the tunnel as a FleetMsg.Data payload. Each daemon→hub
// relay maps a FleetMsg.Type to an ExecEvent.Type: exec_output→output,
// exec_prompt→prompt, exec_result→result, exec_error→error.
type ExecEvent struct {
	Type     string          `json:"type"`
	Stream   string          `json:"stream,omitempty"`
	Data     string          `json:"data,omitempty"`
	Prompt   string          `json:"prompt,omitempty"`
	Awaiting bool            `json:"awaiting,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// Node represents a connected daemon.
type Node struct {
	NodeInfo
	Connected time.Time
	conn      *websocket.Conn
	send      chan []byte
	StreamChan chan string
}

// NewNode creates a new node wrapper.
func NewNode(info NodeInfo, conn *websocket.Conn) *Node {
	return &Node{
		NodeInfo:   info,
		Connected:  time.Now(),
		conn:       conn,
		send:       make(chan []byte, 256),
		StreamChan: make(chan string, 100),
	}
}

// writePump pumps messages from the node's send channel to the websocket connection.
func (n *Node) writePump() {
	defer n.conn.Close()
	for msg := range n.send {
		if err := n.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			fmt.Printf("Failed to write to node %s: %v\n", n.ID, err)
			return
		}
	}
}

// SendMessage enqueues a JSON message to be sent to the node.
func (n *Node) SendMessage(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	
	select {
	case n.send <- b:
		return nil
	default:
		return fmt.Errorf("node %s send queue full", n.ID)
	}
}
