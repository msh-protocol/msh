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
	Arch       string `json:"arch"`
}

type FleetMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
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
