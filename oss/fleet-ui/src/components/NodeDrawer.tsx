import { useState, useEffect } from 'react'
import { ServerIcon, XIcon, CopyIcon, CheckIcon, RadioIcon, TerminalIcon } from './Icons'
import { MshLogo } from './MshLogo'
import './NodeDrawer.css'

export interface NodeDetail {
  id: string
  hostname: string
  os: string
  arch: string
  connected_at?: string
  uptime_sec?: number
}

interface NodeDrawerProps {
  isOpen: boolean
  onClose: () => void
  nodes: NodeDetail[]
  token: string
  onFocusNode?: (nodeId: string) => void
}

function formatUptime(uptimeSec?: number, connectedAt?: string): string {
  if (uptimeSec !== undefined && uptimeSec >= 0) {
    if (uptimeSec < 60) return `${uptimeSec}s`
    const mins = Math.floor(uptimeSec / 60)
    if (mins < 60) return `${mins}m ${uptimeSec % 60}s`
    const hours = Math.floor(mins / 60)
    return `${hours}h ${mins % 60}m`
  }
  if (connectedAt) {
    try {
      const d = new Date(connectedAt)
      const diffSec = Math.floor((Date.now() - d.getTime()) / 1000)
      if (diffSec < 60) return `${diffSec}s`
      return `${Math.floor(diffSec / 60)}m`
    } catch {
      return 'Connected'
    }
  }
  return 'Active'
}

export function NodeDrawer({
  isOpen,
  onClose,
  nodes = [],
  token,
  onFocusNode
}: NodeDrawerProps) {
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [pinging, setPinging] = useState<Record<string, boolean>>({})
  const [pingResults, setPingResults] = useState<Record<string, { latency_ms?: number; online: boolean; error?: string }>>({})

  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  const copyId = (id: string) => {
    navigator.clipboard.writeText(id)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  const pingNode = async (nodeId: string) => {
    setPinging(prev => ({ ...prev, [nodeId]: true }))
    try {
      const res = await fetch(`/api/nodes/ping?id=${encodeURIComponent(nodeId)}`, {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      })
      if (res.ok) {
        const data = await res.json()
        setPingResults(prev => ({
          ...prev,
          [nodeId]: { latency_ms: data.latency_ms ?? 0, online: data.online ?? true, error: data.error }
        }))
      } else {
        setPingResults(prev => ({
          ...prev,
          [nodeId]: { online: false, error: `HTTP ${res.status}` }
        }))
      }
    } catch (err: any) {
      setPingResults(prev => ({
        ...prev,
        [nodeId]: { online: false, error: err?.message || 'Network error' }
      }))
    } finally {
      setPinging(prev => ({ ...prev, [nodeId]: false }))
    }
  }

  if (!isOpen) return null

  return (
    <div className="node-drawer-backdrop" onClick={onClose}>
      <div className="node-drawer" onClick={e => e.stopPropagation()}>
        {/* Header */}
        <div className="node-drawer-header">
          <div className="node-header-left">
            <span className="node-drawer-icon">
              <MshLogo size={22} />
            </span>
            <div>
              <h3 className="node-drawer-title">Connected Swarm Daemons</h3>
              <span className="node-drawer-subtitle">
                {nodes.length} {nodes.length === 1 ? 'daemon' : 'daemons'} registered on fleet hub
              </span>
            </div>
            <div className="node-count-badge mono">
              <span className="dot dot-sage" />
              <span>{nodes.length} Online</span>
            </div>
          </div>
          <div className="node-header-right">
            <button
              type="button"
              className="node-close-btn"
              onClick={onClose}
              title="Close drawer (Esc)"
            >
              <XIcon size={14} />
              <span className="kbd-shortcut mono">ESC</span>
            </button>
          </div>
        </div>

        {/* Content Body */}
        <div className="node-drawer-body">
          {nodes.length === 0 ? (
            <div className="node-empty-state">
              <ServerIcon size={36} className="empty-server-icon" />
              <h4 className="node-empty-heading">No Remote Daemons Connected</h4>
              <p className="node-empty-desc">
                Worker daemons register dynamically when launched with <code>msh serve</code>.
              </p>
              <div className="node-empty-command mono">
                msh serve --fleet ws://127.0.0.1:9000 --token msh-admin-secret
              </div>
            </div>
          ) : (
            <div className="node-cards-list">
              {nodes.map(node => {
                const isPinging = pinging[node.id]
                const pingRes = pingResults[node.id]

                return (
                  <div key={node.id} className="node-specimen-card">
                    <div className="node-card-top">
                      <div className="node-identity">
                        <div className="node-status-dot-wrap">
                          <span className="node-pulse-dot" />
                        </div>
                        <div>
                          <div className="node-hostname-row">
                            <span className="node-hostname">{node.hostname || 'Anonymous Node'}</span>
                            <span className="node-platform-badge mono">
                              {node.os.toUpperCase()} · {node.arch.toUpperCase()}
                            </span>
                          </div>
                          <div className="node-id-row mono">
                            <span className="node-id-label">ID:</span>
                            <span className="node-id-val">{node.id}</span>
                            <button
                              type="button"
                              className="btn-copy-node-id"
                              onClick={() => copyId(node.id)}
                              title="Copy full daemon ID"
                            >
                              {copiedId === node.id ? <CheckIcon size={11} /> : <CopyIcon size={11} />}
                              <span>{copiedId === node.id ? 'Copied' : 'Copy'}</span>
                            </button>
                          </div>
                        </div>
                      </div>

                      <div className="node-uptime-block mono">
                        <span className="uptime-label">UPTIME</span>
                        <span className="uptime-val">{formatUptime(node.uptime_sec, node.connected_at)}</span>
                      </div>
                    </div>

                    {/* Ping / Latency Strip */}
                    <div className="node-ping-strip">
                      <div className="ping-left">
                        <button
                          type="button"
                          className="btn-ping-action mono"
                          onClick={() => pingNode(node.id)}
                          disabled={isPinging}
                        >
                          <RadioIcon size={12} className={isPinging ? 'spin-anim' : ''} />
                          <span>{isPinging ? 'Pinging...' : 'Test Ping'}</span>
                        </button>

                        {pingRes && (
                          <div className={`ping-result-badge mono ${pingRes.online ? 'ping-ok' : 'ping-err'}`}>
                            {pingRes.online ? (
                              <>
                                <span className="dot dot-sage" />
                                <span>{pingRes.latency_ms}ms roundtrip</span>
                              </>
                            ) : (
                              <>
                                <span className="dot dot-terracotta" />
                                <span>{pingRes.error || 'Unreachable'}</span>
                              </>
                            )}
                          </div>
                        )}
                      </div>

                      {onFocusNode && (
                        <button
                          type="button"
                          className="btn-focus-terminal"
                          onClick={() => {
                            onFocusNode(node.id)
                            onClose()
                          }}
                        >
                          <TerminalIcon size={12} />
                          <span>Open Terminal</span>
                        </button>
                      )}
                    </div>
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
