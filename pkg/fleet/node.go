package fleet

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// Node represents a connected msh serve daemon.
type Node struct {
	ID        string
	Hostname  string
	OS        string
	Arch      string
	Connected time.Time
	conn      *websocket.Conn
	send      chan []byte
}

// NodeInfo is the payload a daemon sends when registering.
type NodeInfo struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// NewNode creates a new connected node reference.
func NewNode(info NodeInfo, conn *websocket.Conn) *Node {
	return &Node{
		ID:        info.ID,
		Hostname:  info.Hostname,
		OS:        info.OS,
		Arch:      info.Arch,
		Connected: time.Now(),
		conn:      conn,
		send:      make(chan []byte, 256),
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
