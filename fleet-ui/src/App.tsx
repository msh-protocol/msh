import { useState, useEffect, useRef } from 'react'
import { History } from './components/History'
import { Metrics } from './components/Metrics'

interface NodeInfo {
  id: string
  hostname: string
  os: string
  arch: string
}

function App() {
  const [token, setToken] = useState<string>(() => localStorage.getItem('msh_fleet_token') || '')
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(!!token)
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [activeNode, setActiveNode] = useState<string | null>(null)
  const [logs, setLogs] = useState<string[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const terminalRef = useRef<HTMLDivElement>(null)
  const [currentTab, setCurrentTab] = useState<'fleet' | 'history' | 'metrics'>('fleet')

  // Fetch connected nodes
  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchNodes = async () => {
      try {
        const res = await fetch('/api/nodes', {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        })
        if (res.ok) {
          const data = await res.json()
          setNodes(data || [])
        } else if (res.status === 401) {
          setIsAuthenticated(false)
          localStorage.removeItem('msh_fleet_token')
        }
      } catch (err) {
        console.error("Failed to fetch nodes", err)
      }
    }

    fetchNodes()
    const interval = setInterval(fetchNodes, 2000)
    return () => clearInterval(interval)
  }, [isAuthenticated, token])

  const handleLogin = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const t = formData.get('token') as string
    if (t) {
      setToken(t)
      setIsAuthenticated(true)
      localStorage.setItem('msh_fleet_token', t)
    }
  }

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

  const renderLiveFleet = () => (
    <div className="grid">
      <div className="nodes-panel">
        <div className="panel-header">
          <h2>Connected Daemons</h2>
          <div className="node-count">{nodes.length} Active</div>
        </div>
        <div className="nodes-list">
          {nodes.map(node => (
            <div key={node.id} className={`node-card ${activeNode === node.id ? 'active' : ''}`}>
              <div className="node-info">
                <span className="node-hostname">{node.hostname}</span>
                <span className="node-arch">{node.os}/{node.arch}</span>
              </div>
              <div className="node-id">{node.id}</div>
              <button 
                className="btn-terminal"
                onClick={() => activeNode === node.id ? closeTerminal() : connectTerminal(node.id)}
              >
                {activeNode === node.id ? 'Disconnect' : 'Live Terminal'}
              </button>
            </div>
          ))}
          {nodes.length === 0 && (
            <div className="empty-state">No agents connected.</div>
          )}
        </div>
      </div>

      <div className="terminal-panel">
        <div className="panel-header">
          <h2>Execution Stream</h2>
          {activeNode && <div className="streaming-badge">Live</div>}
        </div>
        <div className="terminal" ref={terminalRef}>
          {logs.length === 0 ? (
            <div className="terminal-empty">Select a daemon to view live execution logs...</div>
          ) : (
            logs.map((log, i) => (
              <div key={i} className="log-line">{log}</div>
            ))
          )}
        </div>
      </div>
    </div>
  )

  return (
    <div className="app-container">
      {!isAuthenticated ? (
        <div className="login-container" style={{ maxWidth: '400px', margin: '4rem auto', textAlign: 'center' }}>
          <h2 className="page-title" style={{ marginBottom: '1rem' }}>Fleet Authentication</h2>
          <p className="empty-desc" style={{ marginBottom: '2rem' }}>Enter the admin token generated by `msh fleet start`</p>
          <form onSubmit={handleLogin} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <input 
              type="text" 
              name="token" 
              placeholder="msh-..." 
              autoComplete="off"
              style={{ 
                padding: '12px', 
                borderRadius: '6px', 
                border: '1px solid var(--border-color)', 
                background: 'var(--panel-bg)',
                color: 'var(--text-primary)',
                fontSize: '14px',
                fontFamily: 'monospace'
              }} 
            />
            <button type="submit" className="btn btn-primary" style={{ height: '40px' }}>
              Authenticate
            </button>
          </form>
        </div>
      ) : (
        <>
          <nav className="navbar">
            <div className="navbar-brand">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 17l6-6-6-6M12 19h8"/></svg>
              msh fleet
            </div>
            <div className="navbar-links">
              <button className={`nav-link ${currentTab === 'fleet' ? 'active' : ''}`} onClick={() => setCurrentTab('fleet')}>Live Fleet</button>
              <button className={`nav-link ${currentTab === 'history' ? 'active' : ''}`} onClick={() => setCurrentTab('history')}>History</button>
              <button className={`nav-link ${currentTab === 'metrics' ? 'active' : ''}`} onClick={() => setCurrentTab('metrics')}>Metrics</button>
            </div>
            <div className="navbar-actions">
              <button className="btn-logout" onClick={() => {
                setToken('')
                setIsAuthenticated(false)
                localStorage.removeItem('msh_fleet_token')
              }}>Logout</button>
            </div>
          </nav>

          <main className="main-content">
            {currentTab === 'fleet' && renderLiveFleet()}
            {currentTab === 'history' && <History token={token} />}
            {currentTab === 'metrics' && <Metrics />}
          </main>
        </>
      )}
    </div>
  )
}

export default App
