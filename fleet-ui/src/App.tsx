import { useState, useEffect, useRef } from 'react'

interface NodeInfo {
  id: string
  hostname: string
  os: string
  arch: string
}

function App() {
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [activeNode, setActiveNode] = useState<string | null>(null)
  const [logs, setLogs] = useState<string[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const terminalRef = useRef<HTMLDivElement>(null)

  // Fetch connected nodes
  useEffect(() => {
    const fetchNodes = async () => {
      try {
        const res = await fetch('/api/nodes')
        if (res.ok) {
          const data = await res.json()
          setNodes(data || [])
        }
      } catch (err) {
        console.error("Failed to fetch nodes", err)
      }
    }

    fetchNodes()
    const interval = setInterval(fetchNodes, 2000)
    return () => clearInterval(interval)
  }, [])

  // Auto-scroll terminal
  useEffect(() => {
    if (terminalRef.current) {
      terminalRef.current.scrollTop = terminalRef.current.scrollHeight
    }
  }, [logs])

  const connectTerminal = (nodeId: string) => {
    setLogs([`Connecting to ${nodeId}...`])
    setActiveNode(nodeId)
    
    // Determine WS URL based on current host
    const loc = window.location
    const wsProto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${wsProto}//${loc.host}/stream/daemon?id=${nodeId}`
    
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setLogs(prev => [...prev, 'Connected to remote shell stream.'])
    }
    
    ws.onmessage = (event) => {
      setLogs(prev => {
        const newLogs = [...prev, event.data]
        // Keep last 200 lines
        if (newLogs.length > 200) return newLogs.slice(newLogs.length - 200)
        return newLogs
      })
    }
    
    ws.onclose = () => {
      setLogs(prev => [...prev, 'Connection closed.'])
    }
  }

  const closeTerminal = () => {
    if (wsRef.current) {
      wsRef.current.close()
    }
    setActiveNode(null)
    setLogs([])
  }

  return (
    <>
      <header className="header">
        <div className="header-brand">
          <div className="brand-logo">m</div>
          <div className="brand-text">msh-fleet</div>
          <div className="brand-badge">Enterprise</div>
        </div>
      </header>

      <main className="main-container">
        <h1 className="page-title">Connected Agents</h1>
        
        {nodes.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">🌐</div>
            <h2 className="empty-title">No Agents Connected</h2>
            <p>Run `msh serve --fleet ws://...` on your agents to connect them to the fleet.</p>
          </div>
        ) : (
          <div className="nodes-grid">
            {nodes.map(node => (
              <div key={node.id} className="node-card">
                <div className="node-header">
                  <div>
                    <div className="node-title">{node.hostname}</div>
                    <div className="node-id">{node.id.substring(0, 18)}...</div>
                  </div>
                  <div className="status-indicator">
                    <div className="status-dot"></div>
                    Online
                  </div>
                </div>
                
                <div className="node-meta">
                  <div className="meta-item">
                    <span className="meta-icon">💻</span>
                    {node.os} ({node.arch})
                  </div>
                  <div className="meta-item">
                    <span className="meta-icon">⏱️</span>
                    Active Session
                  </div>
                </div>
                
                <div className="node-actions">
                  <button className="btn btn-primary" onClick={() => connectTerminal(node.id)}>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="4 17 10 11 4 5"></polyline>
                      <line x1="12" y1="19" x2="20" y2="19"></line>
                    </svg>
                    Live Terminal
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>

      {/* Terminal Modal */}
      {activeNode && (
        <div className="modal-overlay" onClick={(e) => {
          if (e.target === e.currentTarget) closeTerminal()
        }}>
          <div className="terminal-modal">
            <div className="terminal-header">
              <div className="terminal-title">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
                  <line x1="8" y1="21" x2="16" y2="21"></line>
                  <line x1="12" y1="17" x2="12" y2="21"></line>
                </svg>
                {activeNode} — Live Stream
              </div>
              <button className="terminal-close" onClick={closeTerminal}>×</button>
            </div>
            <div className="terminal-body" ref={terminalRef}>
              {logs.map((log, i) => (
                <div key={i} className={`terminal-line ${i === logs.length - 1 ? 'new' : ''}`}>
                  {log}
                </div>
              ))}
              <div className="terminal-cursor"></div>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

export default App
