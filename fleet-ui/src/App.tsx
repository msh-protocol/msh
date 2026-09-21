import { useState, useEffect, useRef } from 'react'
import { History } from './components/History'
import { Metrics } from './components/Metrics'
import { NodeDrawer, type NodeDetail } from './components/NodeDrawer'
import { ShortcutsModal } from './components/ShortcutsModal'
import {
  CopyIcon,
  CheckIcon,
  AlertTriangleIcon,
  XIcon,
  SearchIcon,
  DownloadIcon,
  RadioIcon,
  SendIcon,
  ArrowDownIcon,
  ServerIcon,
  ClockIcon,
  TrendingUpIcon
} from './components/Icons'
import { MshLogo } from './components/MshLogo'

type NodeInfo = NodeDetail

function App() {
  const [token, setToken] = useState<string>(() => localStorage.getItem('msh_fleet_token') || '')
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(!!token)
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [sessions, setSessions] = useState<Record<string, string[]>>({})
  const [closed, setClosed] = useState<Record<string, boolean>>({})
  const wsRefs = useRef<Record<string, WebSocket>>({})
  const tileRefs = useRef<Record<string, HTMLDivElement | null>>({})
  const [currentTab, setCurrentTab] = useState<'fleet' | 'history' | 'metrics'>('fleet')
  const [cmdInputs, setCmdInputs] = useState<Record<string, string>>({})
  const [runningCmds, setRunningCmds] = useState<Record<string, boolean>>({})
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [activePrompts, setActivePrompts] = useState<Record<string, { prompt: string; execWs: WebSocket }>>({})
  const [promptInputs, setPromptInputs] = useState<Record<string, string>>({})

  // Live Fleet Broadcast & Per-Tile Controls State
  const [broadcastCmd, setBroadcastCmd] = useState<string>('')
  const [broadcastTarget, setBroadcastTarget] = useState<string>('all')
  const [isBroadcasting, setIsBroadcasting] = useState<boolean>(false)
  const [broadcastSuccess, setBroadcastSuccess] = useState<boolean>(false)
  const [logFilters, setLogFilters] = useState<Record<string, string>>({})
  const [showFilter, setShowFilter] = useState<Record<string, boolean>>({})
  const [followLogs, setFollowLogs] = useState<Record<string, boolean>>({})
  const [copiedLogs, setCopiedLogs] = useState<Record<string, boolean>>({})

  // Command History State (localStorage backed)
  const [broadcastHistory, setBroadcastHistory] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('msh_broadcast_history')
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })
  const [broadcastHistoryIdx, setBroadcastHistoryIdx] = useState<number>(-1)
  const [broadcastDraft, setBroadcastDraft] = useState<string>('')

  const [nodeHistories, setNodeHistories] = useState<Record<string, string[]>>(() => {
    try {
      const saved = localStorage.getItem('msh_node_histories')
      return saved ? JSON.parse(saved) : {}
    } catch {
      return {}
    }
  })
  const [nodeHistoryIdx, setNodeHistoryIdx] = useState<Record<string, number>>({})
  const [nodeDrafts, setNodeDrafts] = useState<Record<string, string>>({})

  // Drawers & Modals State
  const [isNodeDrawerOpen, setIsNodeDrawerOpen] = useState<boolean>(false)
  const [isShortcutsOpen, setIsShortcutsOpen] = useState<boolean>(false)

  // Swarm Broadcast History Up/Down Handler
  const handleBroadcastKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowUp') {
      if (broadcastHistory.length === 0) return
      e.preventDefault()
      if (broadcastHistoryIdx === -1) {
        setBroadcastDraft(broadcastCmd)
        const newIdx = broadcastHistory.length - 1
        setBroadcastHistoryIdx(newIdx)
        setBroadcastCmd(broadcastHistory[newIdx])
      } else if (broadcastHistoryIdx > 0) {
        const newIdx = broadcastHistoryIdx - 1
        setBroadcastHistoryIdx(newIdx)
        setBroadcastCmd(broadcastHistory[newIdx])
      }
    } else if (e.key === 'ArrowDown') {
      if (broadcastHistoryIdx === -1) return
      e.preventDefault()
      if (broadcastHistoryIdx < broadcastHistory.length - 1) {
        const newIdx = broadcastHistoryIdx + 1
        setBroadcastHistoryIdx(newIdx)
        setBroadcastCmd(broadcastHistory[newIdx])
      } else {
        setBroadcastHistoryIdx(-1)
        setBroadcastCmd(broadcastDraft)
      }
    }
  }

  // Node Tile History Up/Down Handler
  const handleNodeKeyDown = (nodeId: string, e: React.KeyboardEvent<HTMLInputElement>) => {
    const history = nodeHistories[nodeId] || []
    const currentIdx = nodeHistoryIdx[nodeId] ?? -1

    if (e.key === 'ArrowUp') {
      if (history.length === 0) return
      e.preventDefault()
      if (currentIdx === -1) {
        setNodeDrafts(prev => ({ ...prev, [nodeId]: cmdInputs[nodeId] || '' }))
        const newIdx = history.length - 1
        setNodeHistoryIdx(prev => ({ ...prev, [nodeId]: newIdx }))
        setCmdInputs(prev => ({ ...prev, [nodeId]: history[newIdx] }))
      } else if (currentIdx > 0) {
        const newIdx = currentIdx - 1
        setNodeHistoryIdx(prev => ({ ...prev, [nodeId]: newIdx }))
        setCmdInputs(prev => ({ ...prev, [nodeId]: history[newIdx] }))
      }
    } else if (e.key === 'ArrowDown') {
      if (currentIdx === -1) return
      e.preventDefault()
      if (currentIdx < history.length - 1) {
        const newIdx = currentIdx + 1
        setNodeHistoryIdx(prev => ({ ...prev, [nodeId]: newIdx }))
        setCmdInputs(prev => ({ ...prev, [nodeId]: history[newIdx] }))
      } else {
        setNodeHistoryIdx(prev => ({ ...prev, [nodeId]: -1 }))
        setCmdInputs(prev => ({ ...prev, [nodeId]: nodeDrafts[nodeId] || '' }))
      }
    }
  }

  // Global Keyboard Shortcuts (1, 2, 3, /, r, n, ?, Esc)
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      const activeEl = document.activeElement
      const isInputActive =
        activeEl instanceof HTMLInputElement ||
        activeEl instanceof HTMLTextAreaElement ||
        (activeEl as HTMLElement)?.isContentEditable

      if (e.key === 'Escape') {
        if (isNodeDrawerOpen) {
          setIsNodeDrawerOpen(false)
          return
        }
        if (isShortcutsOpen) {
          setIsShortcutsOpen(false)
          return
        }
        if (isInputActive && activeEl instanceof HTMLElement) {
          activeEl.blur()
          return
        }
      }

      // Do not trigger global navigation shortcuts while user is actively typing
      if (isInputActive) return

      if (e.key === '1') {
        e.preventDefault()
        setCurrentTab('fleet')
      } else if (e.key === '2') {
        e.preventDefault()
        setCurrentTab('history')
      } else if (e.key === '3') {
        e.preventDefault()
        setCurrentTab('metrics')
      } else if (e.key === 'n' || e.key === 'N') {
        e.preventDefault()
        setIsNodeDrawerOpen(prev => !prev)
      } else if (e.key === '?') {
        e.preventDefault()
        setIsShortcutsOpen(prev => !prev)
      } else if (e.key === '/') {
        e.preventDefault()
        if (currentTab === 'history') {
          const searchInput = document.querySelector('.search-input') as HTMLInputElement
          if (searchInput) searchInput.focus()
        } else {
          const broadcastInput = document.querySelector('.broadcast-input') as HTMLInputElement
          if (broadcastInput) broadcastInput.focus()
        }
      }
    }

    window.addEventListener('keydown', handleGlobalKeyDown)
    return () => window.removeEventListener('keydown', handleGlobalKeyDown)
  }, [isNodeDrawerOpen, isShortcutsOpen, currentTab])

  const handleBroadcast = (cmdToRun?: string) => {
    const cmd = (cmdToRun || broadcastCmd).trim()
    if (!cmd || isBroadcasting) return

    // Save to command history
    setBroadcastHistory(prev => {
      const filtered = prev.filter(c => c !== cmd)
      const next = [...filtered, cmd].slice(-50)
      try { localStorage.setItem('msh_broadcast_history', JSON.stringify(next)) } catch {}
      return next
    })
    setBroadcastHistoryIdx(-1)
    setBroadcastDraft('')

    setIsBroadcasting(true)
    const targets = broadcastTarget === 'all'
      ? nodes.map(n => n.id)
      : [broadcastTarget]

    targets.forEach(nodeId => {
      if (sessions[nodeId] === undefined) {
        connectTerminal(nodeId)
      }
      setTimeout(() => {
        runCommandOnNode(nodeId, cmd)
      }, 100)
    })

    setBroadcastSuccess(true)
    setTimeout(() => {
      setIsBroadcasting(false)
      setBroadcastSuccess(false)
    }, 1600)
  }

  const copyLogs = (nodeId: string) => {
    const text = (sessions[nodeId] || []).join('\n')
    navigator.clipboard.writeText(text)
    setCopiedLogs(prev => ({ ...prev, [nodeId]: true }))
    setTimeout(() => setCopiedLogs(prev => ({ ...prev, [nodeId]: false })), 2000)
  }

  const downloadLogs = async (nodeId: string) => {
    const text = (sessions[nodeId] || []).join('\n')
    const fileName = `daemon-${nodeId.slice(0, 8)}-logs.txt`

    // Open native folder/file picker if supported
    if ('showSaveFilePicker' in window) {
      try {
        const handle = await (window as any).showSaveFilePicker({
          suggestedName: fileName,
          types: [{
            description: 'Text Log File',
            accept: { 'text/plain': ['.txt', '.log'] }
          }]
        })
        const writable = await handle.createWritable()
        await writable.write(text)
        await writable.close()
        return
      } catch (err: any) {
        if (err.name === 'AbortError') return
        console.warn('showSaveFilePicker failed, falling back to download:', err)
      }
    }

    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = fileName
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

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

    // Save to per-node command history
    setNodeHistories(prev => {
      const list = (prev[nodeId] || []).filter(c => c !== cmd)
      const nextList = [...list, cmd].slice(-50)
      const next = { ...prev, [nodeId]: nextList }
      try { localStorage.setItem('msh_node_histories', JSON.stringify(next)) } catch {}
      return next
    })
    setNodeHistoryIdx(prev => ({ ...prev, [nodeId]: -1 }))
    setNodeDrafts(prev => ({ ...prev, [nodeId]: '' }))

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
              [nodeId]: [...(prev[nodeId] || []), `[Prompt: ${msg.prompt || 'Input required'}]`]
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
    setToken('')
    setIsAuthenticated(false)
    localStorage.removeItem('msh_fleet_token')
  }

  // Auto-scroll each active terminal tile (respecting followLogs setting)
  useEffect(() => {
    Object.keys(sessions).forEach(id => {
      if (followLogs[id] !== false) {
        const el = tileRefs.current[id]
        if (el) el.scrollTop = el.scrollHeight
      }
    })
  }, [sessions, followLogs])

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
  }

  const clearTerminal = (nodeId: string) => {
    setSessions(prev => ({ ...prev, [nodeId]: [] }))
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
  }

  const nodeTitle = (nodeId: string) =>
    nodes.find(n => n.id === nodeId)?.hostname || nodeId

  const liveCount = Object.keys(sessions).filter(isLive).length
  const totalSessions = Object.keys(sessions).length

  const renderLiveFleet = () => {
    const displayedNodes = Object.keys(sessions)

    return (
      <div className="fleet-page-wrapper">
        <div className="editorial-hero">
          <div className="hero-header-flex">
            <div className="hero-emblem-wrap">
              <MshLogo size={46} className="hero-msh-logo" />
            </div>
            <div className="hero-header-text">
              <div className="hero-kicker-row">
                <span className="hero-kicker-pill">~ SRE fleet gauntlet ~</span>
                <span className="hero-kicker-text">Live Daemon Telemetry · Parallel Terminal Grid</span>
              </div>
              <h1 className="hero-headline">Autonomous Fleet Orchestration & Telemetry</h1>
              <p className="hero-subtext">
                Deterministic execution monitoring, auto-scaling daemons, and concurrent stream traces across your active nodes.
              </p>
            </div>
          </div>
        </div>

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
                    <span className="copy-label">{copiedId === node.id ? <CheckIcon size={12} /> : <CopyIcon size={12} />}</span>
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

          {/* Swarm Broadcast Command Bar */}
          <div className="broadcast-bar">
            <div className="broadcast-header">
              <div className="broadcast-title">
                <RadioIcon size={15} className={`broadcast-icon ${isBroadcasting ? 'pulse-icon' : ''}`} />
                <span>Swarm Broadcast</span>
                <span className="broadcast-badge">{nodes.length} node{nodes.length === 1 ? '' : 's'} available</span>
              </div>
              <div className="broadcast-target-picker">
                <label htmlFor="broadcast-target-select">Target:</label>
                <select
                  id="broadcast-target-select"
                  value={broadcastTarget}
                  onChange={(e) => setBroadcastTarget(e.target.value)}
                  className="broadcast-select"
                >
                  <option value="all">All Daemons ({nodes.length})</option>
                  {nodes.map(n => (
                    <option key={n.id} value={n.id}>
                      {n.hostname} ({n.id.slice(0, 8)})
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <form
              className="broadcast-form"
              onSubmit={(e) => {
                e.preventDefault()
                handleBroadcast()
              }}
            >
              <div className="broadcast-input-wrap">
                <span className="broadcast-prompt">$</span>
                <input
                  type="text"
                  className="broadcast-input"
                  placeholder="Broadcast command across selected daemons concurrently... (↑/↓ for history)"
                  value={broadcastCmd}
                  onChange={(e) => setBroadcastCmd(e.target.value)}
                  onKeyDown={handleBroadcastKeyDown}
                  disabled={nodes.length === 0}
                />
              </div>
              <button
                type="submit"
                className={`btn-broadcast ${broadcastSuccess ? 'btn-success' : ''}`}
                disabled={isBroadcasting || !broadcastCmd.trim() || nodes.length === 0}
              >
                {isBroadcasting ? (
                  <>
                    <RadioIcon size={13} className="icon-spin" />
                    <span>Dispatching...</span>
                  </>
                ) : broadcastSuccess ? (
                  <>
                    <CheckIcon size={13} />
                    <span>Dispatched!</span>
                  </>
                ) : (
                  <>
                    <SendIcon size={13} />
                    <span>Broadcast</span>
                  </>
                )}
              </button>
            </form>

            <div className="broadcast-presets">
              <span className="presets-label">Quick Presets:</span>
              {['git status', 'uptime', 'whoami', 'df -h', 'docker ps'].map(preset => (
                <button
                  key={preset}
                  type="button"
                  className="btn-preset"
                  onClick={() => {
                    setBroadcastCmd(preset)
                    handleBroadcast(preset)
                  }}
                  disabled={nodes.length === 0}
                >
                  {preset}
                </button>
              ))}
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
            <div className={`terminal-grid ${displayedNodes.length === 1 ? 'single-item' : ''}`}>
              {displayedNodes.map(nodeId => {
                const live = isLive(nodeId)
                const rawLogs = sessions[nodeId] || []
                const lineCount = rawLogs.length
                const filterQuery = (logFilters[nodeId] || '').trim().toLowerCase()
                const displayedLogs = filterQuery
                  ? rawLogs.filter(log => log.toLowerCase().includes(filterQuery))
                  : rawLogs
                const isFilterActive = showFilter[nodeId]
                const isFollowing = followLogs[nodeId] !== false

                return (
                  <div
                    key={nodeId}
                    className={`terminal-tile ${live ? 'is-live' : 'is-closed'}`}
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
                            <span>{nodeId}</span>
                            {copiedId === nodeId && <CheckIcon size={11} className="tile-copy-icon" />}
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

                        {/* In-Terminal Filter Toggle */}
                        <button
                          className={`btn-tile-action ${isFilterActive ? 'active' : ''}`}
                          onClick={() => setShowFilter(prev => ({ ...prev, [nodeId]: !prev[nodeId] }))}
                          title="Search and filter output lines"
                        >
                          <SearchIcon size={12} />
                          <span>Filter</span>
                        </button>

                        {/* Follow Logs / Scroll Lock Toggle */}
                        <button
                          className={`btn-tile-action ${isFollowing ? 'active' : ''}`}
                          onClick={() => setFollowLogs(prev => ({ ...prev, [nodeId]: prev[nodeId] === false }))}
                          title={isFollowing ? "Auto-scroll ON: clicks will pause auto-scroll" : "Auto-scroll PAUSED: click to follow new output"}
                        >
                          <ArrowDownIcon size={12} />
                          <span>{isFollowing ? 'Follow' : 'Hold'}</span>
                        </button>

                        {/* Copy Logs */}
                        <button
                          className="btn-tile-action"
                          onClick={() => copyLogs(nodeId)}
                          title="Copy all logs to clipboard"
                        >
                          {copiedLogs[nodeId] ? <CheckIcon size={12} /> : <CopyIcon size={12} />}
                          <span>{copiedLogs[nodeId] ? 'Copied' : 'Copy'}</span>
                        </button>

                        {/* Download Logs */}
                        <button
                          className="btn-tile-action"
                          onClick={() => downloadLogs(nodeId)}
                          title="Download logs as .txt file"
                        >
                          <DownloadIcon size={12} />
                        </button>

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
                          className="btn-tile-close"
                          onClick={() => closeTerminal(nodeId)}
                          title="Close terminal tile"
                        >
                          <XIcon size={13} />
                        </button>
                      </div>
                    </div>

                    {/* Inline Filter Bar */}
                    {isFilterActive && (
                      <div className="tile-filter-bar">
                        <SearchIcon size={12} className="tile-filter-icon" />
                        <input
                          type="text"
                          className="tile-filter-input"
                          placeholder="Filter log lines by keyword..."
                          value={logFilters[nodeId] || ''}
                          onChange={(e) => setLogFilters(prev => ({ ...prev, [nodeId]: e.target.value }))}
                          autoFocus
                        />
                        {logFilters[nodeId] && (
                          <button
                            className="btn-clear-filter"
                            onClick={() => setLogFilters(prev => ({ ...prev, [nodeId]: '' }))}
                            title="Clear filter query"
                          >
                            <XIcon size={11} />
                          </button>
                        )}
                      </div>
                    )}

                    <div
                      className="terminal tile-terminal"
                      ref={el => { tileRefs.current[nodeId] = el }}
                    >
                      {lineCount === 0 ? (
                        <div className="terminal-waiting">Waiting for execution output...</div>
                      ) : filterQuery && displayedLogs.length === 0 ? (
                        <div className="terminal-filter-empty">
                          No lines match filter "{logFilters[nodeId]}" ({lineCount} total lines)
                        </div>
                      ) : (
                        displayedLogs.map((log, i) => (
                          <div key={i} className="log-line">{log}</div>
                        ))
                      )}
                    </div>
                    {activePrompts[nodeId] && (
                      <div className="tile-prompt-banner">
                        <div className="prompt-header">
                          <span className="prompt-icon">
                            <AlertTriangleIcon size={14} />
                          </span>
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
                        placeholder={`Execute command on ${nodeTitle(nodeId)} (↑/↓ for history)...`}
                        value={cmdInputs[nodeId] || ''}
                        onChange={(e) => setCmdInputs(prev => ({ ...prev, [nodeId]: e.target.value }))}
                        onKeyDown={(e) => handleNodeKeyDown(nodeId, e)}
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
      </div>
    )
  }

  return (
    <div className="app-container">
      {!isAuthenticated ? (
        <div className="login-container" style={{ maxWidth: '400px', margin: '4rem auto', textAlign: 'center' }}>
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '1.25rem' }}>
            <MshLogo size={56} />
          </div>
          <h2 className="page-title" style={{ marginBottom: '0.5rem' }}>Fleet Authentication</h2>
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
              <div className="brand-logo-badge">
                <MshLogo size={28} />
              </div>
              <span className="brand-title">msh fleet</span>
              <span className="brand-pill">~ swarm 1.4 ~</span>
            </div>

            <div className="navbar-center">
              <div className="nav-pill-group">
                <button
                  className={`nav-pill ${currentTab === 'fleet' ? 'active' : ''}`}
                  onClick={() => setCurrentTab('fleet')}
                >
                  <ServerIcon size={13} />
                  <span>Live Fleet</span>
                </button>
                <button
                  className={`nav-pill ${currentTab === 'history' ? 'active' : ''}`}
                  onClick={() => setCurrentTab('history')}
                >
                  <ClockIcon size={13} />
                  <span>History</span>
                </button>
                <button
                  className={`nav-pill ${currentTab === 'metrics' ? 'active' : ''}`}
                  onClick={() => setCurrentTab('metrics')}
                >
                  <TrendingUpIcon size={13} />
                  <span>Metrics</span>
                </button>
              </div>
            </div>

            <div className="navbar-actions">
              <button
                type="button"
                className="cluster-status-pill cluster-status-btn"
                onClick={() => setIsNodeDrawerOpen(true)}
                title="View Connected Swarm Daemons (Press 'n')"
              >
                <span className="status-live-dot" />
                <span>{nodes.length} {nodes.length === 1 ? 'node' : 'nodes'} online</span>
              </button>
              <button
                type="button"
                className="btn-kbd-help"
                onClick={() => setIsShortcutsOpen(true)}
                title="Keyboard Shortcuts (Press '?')"
              >
                <span className="mono">?</span>
              </button>
              <button className="btn-logout" onClick={handleLogout}>Logout</button>
            </div>
          </nav>

          <main className="main-content">
            {currentTab === 'fleet' && renderLiveFleet()}
            {currentTab === 'history' && <History token={token} nodes={nodes} onRerun={handleRerunCommand} />}
            {currentTab === 'metrics' && <Metrics token={token} />}
          </main>

          <NodeDrawer
            isOpen={isNodeDrawerOpen}
            onClose={() => setIsNodeDrawerOpen(false)}
            nodes={nodes}
            token={token}
            onFocusNode={(nodeId) => {
              setCurrentTab('fleet')
              if (sessions[nodeId] === undefined) {
                connectTerminal(nodeId)
              }
            }}
          />
          <ShortcutsModal
            isOpen={isShortcutsOpen}
            onClose={() => setIsShortcutsOpen(false)}
          />
        </>
      )}
    </div>
  )
}

export default App
