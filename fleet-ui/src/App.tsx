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
  const [sessions, setSessions] = useState<Record<string, string[]>>({})
  const [closed, setClosed] = useState<Record<string, boolean>>({})
  const [maximizedNode, setMaximizedNode] = useState<string | null>(null)
  const [gridLayout, setGridLayout] = useState<'auto' | '1' | '2' | '3'>('auto')
  const wsRefs = useRef<Record<string, WebSocket>>({})
  const tileRefs = useRef<Record<string, HTMLDivElement | null>>({})
  const [currentTab, setCurrentTab] = useState<'fleet' | 'history' | 'metrics'>('fleet')
  const [cmdInputs, setCmdInputs] = useState<Record<string, string>>({})
  const [runningCmds, setRunningCmds] = useState<Record<string, boolean>>({})
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [activePrompts, setActivePrompts] = useState<Record<string, { prompt: string; execWs: WebSocket }>>({})
  const [promptInputs, setPromptInputs] = useState<Record<string, string>>({})

  const copyNodeId = (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    navigator.clipboard.writeText(id)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  const sendPromptAnswer = (nodeId: string, answer: string) => {
    const p = activePrompts[nodeId]
    if (!p || !p.execWs) return

    p.execWs.send(JSON.stringify({
      type: 'answer',
      data: answer
    }))

    setSessions(prev => ({
      ...prev,
      [nodeId]: [...(prev[nodeId] || []), `[Prompt Answered: "${answer}"]`]
    }))

    setActivePrompts(prev => {
      const next = { ...prev }
      delete next[nodeId]
      return next
    })
    setPromptInputs(prev => ({ ...prev, [nodeId]: '' }))
  }

  const handleRerunCommand = (nodeId: string, cmd: string) => {
    if (!cmd) return
    setCurrentTab('fleet')
    if (!isLive(nodeId)) {
      connectTerminal(nodeId)
    }
    setTimeout(() => {
      runCommandOnNode(nodeId, cmd)
    }, 150)
  }

  const runCommandOnNode = (nodeId: string, cmd: string) => {
    if (!cmd.trim() || runningCmds[nodeId]) return
    setRunningCmds(prev => ({ ...prev, [nodeId]: true }))

    const loc = window.location
    const wsProto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
    const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
    const execWsUrl = `${wsProto}//${loc.host}/stream/exec?id=${encodeURIComponent(nodeId)}${tokenParam}`

    setSessions(prev => ({
      ...prev,
      [nodeId]: [...(prev[nodeId] || []), `\n$ ${cmd}`]
    }))

    try {
      const execWs = new WebSocket(execWsUrl)
      execWs.onopen = () => {
        execWs.send(JSON.stringify({
          type: 'start',
          request: { command: cmd, max_output_lines: 500 }
        }))
      }
      execWs.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'output' && msg.data) {
            setSessions(prev => ({
              ...prev,
              [nodeId]: [...(prev[nodeId] || []), msg.data]
            }))
          } else if (msg.type === 'prompt' && msg.awaiting) {
            setActivePrompts(prev => ({
              ...prev,
              [nodeId]: { prompt: msg.prompt || 'Input required:', execWs }
            }))
            setSessions(prev => ({
              ...prev,
              [nodeId]: [...(prev[nodeId] || []), `[⚠️ Prompt: ${msg.prompt || 'Input required'}]`]
            }))
          } else if (msg.type === 'result') {
            setActivePrompts(prev => {
              const next = { ...prev }
              delete next[nodeId]
              return next
            })
            const res = typeof msg.response === 'string' ? JSON.parse(msg.response) : msg.response
            if (res && res.stdout) {
              setSessions(prev => ({
                ...prev,
                [nodeId]: [...(prev[nodeId] || []), res.stdout]
              }))
            }
            if (res && res.stderr) {
              setSessions(prev => ({
                ...prev,
                [nodeId]: [...(prev[nodeId] || []), `[stderr] ${res.stderr}`]
              }))
            }
            setSessions(prev => ({
              ...prev,
              [nodeId]: [...(prev[nodeId] || []), `[exit: ${res?.exit_code ?? 0}]`]
            }))
          } else if (msg.type === 'error') {
            setActivePrompts(prev => {
              const next = { ...prev }
              delete next[nodeId]
              return next
            })
            setSessions(prev => ({
              ...prev,
              [nodeId]: [...(prev[nodeId] || []), `[error] ${msg.error || msg.data}`]
            }))
          }
        } catch {
          setSessions(prev => ({
            ...prev,
            [nodeId]: [...(prev[nodeId] || []), event.data]
          }))
        }
      }
      execWs.onclose = () => {
        setRunningCmds(prev => ({ ...prev, [nodeId]: false }))
        setActivePrompts(prev => {
          const next = { ...prev }
          delete next[nodeId]
          return next
        })
      }
      execWs.onerror = () => {
        setSessions(prev => ({
          ...prev,
          [nodeId]: [...(prev[nodeId] || []), '[exec connection failed]']
        }))
        setRunningCmds(prev => ({ ...prev, [nodeId]: false }))
        setActivePrompts(prev => {
          const next = { ...prev }
          delete next[nodeId]
          return next
        })
      }
    } catch {
      setRunningCmds(prev => ({ ...prev, [nodeId]: false }))
    }
  }

  // Clean up all WebSockets on unmount
  useEffect(() => {
    return () => {
      Object.values(wsRefs.current).forEach(ws => ws.close())
      wsRefs.current = {}
    }
  }, [])

  // Fetch connected nodes
  useEffect(() => {
    if (!isAuthenticated) return

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

  const [loginError, setLoginError] = useState<string | null>(null)
  const [isAuthenticating, setIsAuthenticating] = useState<boolean>(false)

  const handleLogin = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setLoginError(null)
    const formData = new FormData(e.currentTarget)
    const t = ((formData.get('token') as string) || '').trim()
    if (!t) {
      setLoginError("Please enter the admin token generated by `msh fleet start`.")
      return
    }

    setIsAuthenticating(true)
    try {
      const res = await fetch('/api/nodes', {
        headers: {
          'Authorization': `Bearer ${t}`
        }
      })
      if (res.ok) {
        const data = await res.json()
        setToken(t)
        setIsAuthenticated(true)
        setNodes(data || [])
        localStorage.setItem('msh_fleet_token', t)
      } else if (res.status === 401) {
        setLoginError("Invalid admin token. Note: Enter the `msh-...` token printed in your terminal when running `msh fleet start` (not your machine/OS password).")
      } else {
        setLoginError(`Authentication failed (HTTP ${res.status}).`)
      }
    } catch {
      setLoginError("Unable to reach the fleet hub. Please verify that `msh fleet start` is running.")
    } finally {
      setIsAuthenticating(false)
    }
  }

  const handleLogout = () => {
    Object.values(wsRefs.current).forEach(ws => ws.close())
    wsRefs.current = {}
    setSessions({})
    setClosed({})
    setMaximizedNode(null)
    setToken('')
    setIsAuthenticated(false)
    localStorage.removeItem('msh_fleet_token')
  }

  // Auto-scroll each active terminal tile
  useEffect(() => {
    Object.keys(sessions).forEach(id => {
      const el = tileRefs.current[id]
      if (el) el.scrollTop = el.scrollHeight
    })
  }, [sessions])

  const isLive = (nodeId: string) =>
    sessions[nodeId] !== undefined && !closed[nodeId]

  const connectTerminal = (nodeId: string) => {
    if (wsRefs.current[nodeId]) {
      wsRefs.current[nodeId].close()
      delete wsRefs.current[nodeId]
    }

    const loc = window.location
    const wsProto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
    const tokenParam = token ? `&token=${encodeURIComponent(token)}` : ''
    const wsUrl = `${wsProto}//${loc.host}/stream/daemon?id=${encodeURIComponent(nodeId)}${tokenParam}`

    const ws = new WebSocket(wsUrl)
    wsRefs.current[nodeId] = ws

    setSessions(prev => ({
      ...prev,
      [nodeId]: prev[nodeId] && prev[nodeId].length > 0
        ? [...prev[nodeId], `--- Reconnecting to ${nodeId} ---`]
        : [`Connecting to ${nodeId}...`]
    }))
    setClosed(prev => ({ ...prev, [nodeId]: false }))

    ws.onopen = () => {
      setSessions(prev => ({
        ...prev,
        [nodeId]: [...(prev[nodeId] || []), 'Connected to remote shell stream.']
      }))
    }

    ws.onmessage = (event) => {
      setSessions(prev => {
        const current = prev[nodeId] || []
        const next = [...current, event.data]
        return { ...prev, [nodeId]: next.length > 500 ? next.slice(next.length - 500) : next }
      })
    }

    ws.onerror = () => {
      setSessions(prev => ({
        ...prev,
        [nodeId]: [...(prev[nodeId] || []), 'Connection error.']
      }))
    }

    ws.onclose = () => {
      delete wsRefs.current[nodeId]
      setSessions(prev => ({
        ...prev,
        [nodeId]: [...(prev[nodeId] || []), 'Connection closed.']
      }))
      setClosed(prev => ({ ...prev, [nodeId]: true }))
    }
  }

  const disconnectTerminal = (nodeId: string) => {
    if (wsRefs.current[nodeId]) {
      wsRefs.current[nodeId].close()
      delete wsRefs.current[nodeId]
    }
    setClosed(prev => ({ ...prev, [nodeId]: true }))
  }

  const closeTerminal = (nodeId: string) => {
    if (wsRefs.current[nodeId]) {
      wsRefs.current[nodeId].close()
      delete wsRefs.current[nodeId]
    }
    setSessions(prev => {
      const next = { ...prev }
      delete next[nodeId]
      return next
    })
    setClosed(prev => {
      const next = { ...prev }
      delete next[nodeId]
      return next
    })
    if (maximizedNode === nodeId) {
      setMaximizedNode(null)
    }
  }

  const clearTerminal = (nodeId: string) => {
    setSessions(prev => ({ ...prev, [nodeId]: [] }))
  }

  const toggleMaximize = (nodeId: string) => {
    setMaximizedNode(prev => (prev === nodeId ? null : nodeId))
  }

  const connectAll = () => {
    nodes.forEach(node => {
      if (!isLive(node.id)) {
        connectTerminal(node.id)
      }
    })
  }

  const disconnectAll = () => {
    Object.keys(wsRefs.current).forEach(id => {
      wsRefs.current[id]?.close()
      delete wsRefs.current[id]
    })
    setClosed(prev => {
      const next: Record<string, boolean> = { ...prev }
      Object.keys(sessions).forEach(id => {
        next[id] = true
      })
      return next
    })
  }

  const clearAll = () => {
    setSessions(prev => {
      const next: Record<string, string[]> = {}
      Object.keys(prev).forEach(id => {
        next[id] = []
      })
      return next
    })
  }

  const closeAll = () => {
    Object.keys(wsRefs.current).forEach(id => {
      wsRefs.current[id]?.close()
      delete wsRefs.current[id]
    })
    setSessions({})
    setClosed({})
    setMaximizedNode(null)
  }

  const nodeTitle = (nodeId: string) =>
    nodes.find(n => n.id === nodeId)?.hostname || nodeId

  const liveCount = Object.keys(sessions).filter(isLive).length
  const totalSessions = Object.keys(sessions).length

  const renderLiveFleet = () => {
    const displayedNodes = maximizedNode
      ? [maximizedNode].filter(id => sessions[id] !== undefined)
      : Object.keys(sessions)

    return (
      <div className="fleet-layout">
        <div className="nodes-panel">
          <div className="panel-header">
            <div>
              <h2>Connected Daemons</h2>
              <div className="panel-subtitle">Registered execution nodes</div>
            </div>
            <div className="node-count">{nodes.length} Active</div>
          </div>
          {nodes.length > 0 && (
            <div className="nodes-actions-bar">
              <button
                className="btn-text"
                onClick={connectAll}
                disabled={liveCount === nodes.length}
              >
                Connect All
              </button>
              {liveCount > 0 && (
                <button className="btn-text" onClick={disconnectAll}>
                  Disconnect All
                </button>
              )}
            </div>
          )}
          <div className="nodes-list">
            {nodes.map(node => {
              const live = isLive(node.id)
              const isOpen = sessions[node.id] !== undefined
              return (
                <div key={node.id} className={`node-card ${live ? 'active' : ''}`}>
                  <div className="node-info">
                    <span className="node-hostname">{node.hostname}</span>
                    <span className="node-arch">{node.os}/{node.arch}</span>
                  </div>
                  <div
                    className="node-id"
                    onClick={(e) => copyNodeId(node.id, e)}
                    title="Click to copy node ID"
                  >
                    <span className="node-id-text">{node.id}</span>
                    <span className="copy-label">{copiedId === node.id ? '✓' : '⧉'}</span>
                  </div>
                  <div className="node-card-actions">
                    <button
                      className={`btn-terminal ${live ? 'btn-live' : ''}`}
                      onClick={() => live ? disconnectTerminal(node.id) : connectTerminal(node.id)}
                    >
                      {live ? 'Disconnect' : (isOpen ? 'Reconnect' : 'Live Terminal')}
                    </button>
                  </div>
                </div>
              )
            })}
            {nodes.length === 0 && (
              <div className="empty-state">No agents connected.</div>
            )}
          </div>
        </div>

        <div className="terminal-grid-wrapper">
          <div className="terminal-toolbar">
            <div className="toolbar-left">
              <div className="toolbar-title-group">
                <h3>Terminal Grid</h3>
                <span className="badge-count">
                  {liveCount} Live {totalSessions > 0 && `· ${totalSessions} Open`}
                </span>
              </div>
            </div>

            <div className="toolbar-right">
              {totalSessions > 0 && !maximizedNode && (
                <div className="layout-picker">
                  <span className="layout-label">Layout:</span>
                  <button
                    className={`btn-layout ${gridLayout === 'auto' ? 'active' : ''}`}
                    onClick={() => setGridLayout('auto')}
                    title="Auto Grid"
                  >
                    Auto
                  </button>
                  <button
                    className={`btn-layout ${gridLayout === '1' ? 'active' : ''}`}
                    onClick={() => setGridLayout('1')}
                    title="1 Column"
                  >
                    1 Col
                  </button>
                  <button
                    className={`btn-layout ${gridLayout === '2' ? 'active' : ''}`}
                    onClick={() => setGridLayout('2')}
                    title="2 Columns"
                  >
                    2 Col
                  </button>
                  <button
                    className={`btn-layout ${gridLayout === '3' ? 'active' : ''}`}
                    onClick={() => setGridLayout('3')}
                    title="3 Columns"
                  >
                    3 Col
                  </button>
                </div>
              )}

              {maximizedNode && (
                <button
                  className="btn-toolbar btn-restore"
                  onClick={() => setMaximizedNode(null)}
                >
                  Restore Grid
                </button>
              )}

              {totalSessions > 0 && (
                <>
                  <button className="btn-toolbar" onClick={clearAll} title="Clear all terminal logs">
                    Clear All
                  </button>
                  <button className="btn-toolbar" onClick={closeAll} title="Close all terminals">
                    Close All
                  </button>
                </>
              )}
            </div>
          </div>

          {totalSessions === 0 ? (
            <div className="terminal-empty-container">
              <div className="terminal-empty-icon">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
                  <line x1="8" y1="21" x2="16" y2="21"/>
                  <line x1="12" y1="17" x2="12" y2="21"/>
                  <path d="M7 8l3 3-3 3M13 14h4"/>
                </svg>
              </div>
              <h4>No Active Terminals</h4>
              <p>Select a daemon from the list on the left to start streaming live execution output, or stream all agents simultaneously.</p>
              {nodes.length > 0 && (
                <button className="btn btn-primary" onClick={connectAll} style={{ marginTop: '16px' }}>
                  Connect All Daemons ({nodes.length})
                </button>
              )}
            </div>
          ) : (
            <div className={`terminal-grid layout-${gridLayout} ${maximizedNode ? 'has-maximized' : ''} ${displayedNodes.length === 1 && !maximizedNode ? 'single-item' : ''}`}>
              {displayedNodes.map(nodeId => {
                const live = isLive(nodeId)
                const isMax = maximizedNode === nodeId
                const lineCount = (sessions[nodeId] || []).length
                return (
                  <div
                    key={nodeId}
                    className={`terminal-tile ${isMax ? 'maximized' : ''} ${live ? 'is-live' : 'is-closed'}`}
                  >
                    <div className="panel-header tile-header">
                      <div className="tile-title">
                        <div className="tile-title-row">
                          <span className={`status-dot ${live ? 'live' : 'closed'}`} />
                          <h2>{nodeTitle(nodeId)}</h2>
                          <span className="line-count-badge">{lineCount} lines</span>
                          <span
                            className="tile-node-id"
                            onClick={(e) => copyNodeId(nodeId, e)}
                            title="Click to copy node ID"
                          >
                            {nodeId} {copiedId === nodeId ? '✓' : ''}
                          </span>
                        </div>
                      </div>
                      <div className="tile-actions">
                        {live ? (
                          <span className="streaming-badge">
                            <span className="pulse-dot" /> Live
                          </span>
                        ) : (
                          <span className="closed-badge">Paused</span>
                        )}
                        <button
                          className="btn-tile-action"
                          onClick={() => clearTerminal(nodeId)}
                          title="Clear logs"
                        >
                          Clear
                        </button>
                        {live ? (
                          <button
                            className="btn-tile-action"
                            onClick={() => disconnectTerminal(nodeId)}
                            title="Pause stream"
                          >
                            Pause
                          </button>
                        ) : (
                          <button
                            className="btn-tile-action btn-reconnect"
                            onClick={() => connectTerminal(nodeId)}
                            title="Reconnect to stream"
                          >
                            Reconnect
                          </button>
                        )}
                        <button
                          className="btn-tile-action"
                          onClick={() => toggleMaximize(nodeId)}
                          title={isMax ? "Restore tile" : "Maximize tile"}
                        >
                          {isMax ? 'Restore' : 'Expand'}
                        </button>
                        <button
                          className="btn-tile-close"
                          onClick={() => closeTerminal(nodeId)}
                          title="Close terminal tile"
                        >
                          ✕
                        </button>
                      </div>
                    </div>
                    <div
                      className="terminal tile-terminal"
                      ref={el => { tileRefs.current[nodeId] = el }}
                    >
                      {lineCount === 0 ? (
                        <div className="terminal-waiting">Waiting for execution output...</div>
                      ) : (
                        (sessions[nodeId] || []).map((log, i) => (
                          <div key={i} className="log-line">{log}</div>
                        ))
                      )}
                    </div>
                    {activePrompts[nodeId] && (
                      <div className="tile-prompt-banner">
                        <div className="prompt-header">
                          <span className="prompt-icon">⚠️</span>
                          <span className="prompt-title">Interactive Prompt Detected</span>
                          <span className="prompt-pulse" />
                        </div>
                        <div className="prompt-message">
                          {activePrompts[nodeId].prompt}
                        </div>
                        <div className="prompt-actions-container">
                          {/\[y\/n\]|\(y\/n\)|proceed\?|continue\?|y\/n/i.test(activePrompts[nodeId].prompt) && (
                            <div className="prompt-quick-group">
                              <button
                                type="button"
                                className="btn-quick-ans btn-yes"
                                onClick={() => sendPromptAnswer(nodeId, 'y')}
                              >
                                Yes (y)
                              </button>
                              <button
                                type="button"
                                className="btn-quick-ans btn-no"
                                onClick={() => sendPromptAnswer(nodeId, 'n')}
                              >
                                No (n)
                              </button>
                            </div>
                          )}
                          <form
                            className="prompt-input-form"
                            onSubmit={(e) => {
                              e.preventDefault()
                              sendPromptAnswer(nodeId, promptInputs[nodeId] || '')
                            }}
                          >
                            <input
                              type="text"
                              className="prompt-text-input"
                              autoFocus
                              placeholder="Type answer and press Enter..."
                              value={promptInputs[nodeId] || ''}
                              onChange={(e) => setPromptInputs(prev => ({ ...prev, [nodeId]: e.target.value }))}
                            />
                            <button
                              type="submit"
                              className="btn-send-answer"
                            >
                              Send Answer ↵
                            </button>
                          </form>
                        </div>
                      </div>
                    )}
                    <form
                      className="tile-exec-bar"
                      onSubmit={(e) => {
                        e.preventDefault()
                        const cmd = cmdInputs[nodeId] || ''
                        if (cmd.trim()) {
                          runCommandOnNode(nodeId, cmd)
                          setCmdInputs(prev => ({ ...prev, [nodeId]: '' }))
                        }
                      }}
                    >
                      <span className="tile-exec-prompt">$</span>
                      <input
                        type="text"
                        className="tile-exec-input"
                        placeholder={`Execute command on ${nodeTitle(nodeId)} (e.g. dir, git status, whoami)...`}
                        value={cmdInputs[nodeId] || ''}
                        onChange={(e) => setCmdInputs(prev => ({ ...prev, [nodeId]: e.target.value }))}
                      />
                      <button
                        type="submit"
                        className="tile-exec-btn"
                        disabled={!(cmdInputs[nodeId] || '').trim() || runningCmds[nodeId]}
                      >
                        {runningCmds[nodeId] ? 'Running...' : 'Execute'}
                      </button>
                    </form>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    )
  }

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
            {loginError && (
              <div style={{
                padding: '10px 14px',
                borderRadius: '6px',
                backgroundColor: 'rgba(255, 0, 0, 0.1)',
                border: '1px solid rgba(255, 0, 0, 0.3)',
                color: '#ff6b6b',
                fontSize: '13px',
                lineHeight: '1.4',
                textAlign: 'left'
              }}>
                {loginError}
              </div>
            )}
            <button
              type="submit"
              className="btn btn-primary"
              style={{ height: '40px' }}
              disabled={isAuthenticating}
            >
              {isAuthenticating ? 'Authenticating...' : 'Authenticate'}
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
              <button className="btn-logout" onClick={handleLogout}>Logout</button>
            </div>
          </nav>

          <main className="main-content">
            {currentTab === 'fleet' && renderLiveFleet()}
            {currentTab === 'history' && <History token={token} nodes={nodes} onRerun={handleRerunCommand} />}
            {currentTab === 'metrics' && <Metrics token={token} />}
          </main>
        </>
      )}
    </div>
  )
}

export default App
