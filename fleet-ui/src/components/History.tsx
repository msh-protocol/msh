import { useState, useEffect, useCallback } from 'react'

interface ExecutionRecord {
  ID: number
  Timestamp: string
  SessionID: string
  Command: string
  Status: string
  ExitCode: number
  DurationMs: number
  ReqJSON: string
  RespJSON: string
}

const PAGE_SIZE = 50

export interface NodeInfo {
  id: string
  hostname: string
  os: string
  arch: string
}

export function History({ token, nodes = [], onRerun }: {
  token: string
  nodes?: NodeInfo[]
  onRerun?: (nodeId: string, command: string) => void
}) {
  const [records, setRecords] = useState<ExecutionRecord[]>([])
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [expanded, setExpanded] = useState<number | null>(null)

  const fetchHistory = useCallback(async () => {
    try {
      const res = await fetch(`/api/history?limit=${PAGE_SIZE}&offset=${offset}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        const data = await res.json()
        setRecords(data || [])
        const t = Number(res.headers.get('X-Total-Count'))
        setTotal(Number.isFinite(t) ? t : (data || []).length)
      }
    } catch (err) {
      console.error("Failed to fetch history", err)
    }
  }, [token, offset])

  useEffect(() => {
    fetchHistory()
    const interval = setInterval(() => {
      if (offset === 0) fetchHistory()
    }, 5000)
    return () => clearInterval(interval)
  }, [fetchHistory, offset])

  const currentPage = Math.floor(offset / PAGE_SIZE) + 1
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const firstOnPage = total === 0 ? 0 : offset + 1
  const lastOnPage = Math.min(offset + PAGE_SIZE, total)

  const goToPage = (page: number) => {
    const clamped = Math.min(Math.max(1, page), totalPages)
    setExpanded(null)
    setOffset((clamped - 1) * PAGE_SIZE)
  }

  return (
    <div className="history-container">
      <h2>Execution History</h2>
      <p className="subtitle">Real-time log of all commands executed across the fleet.</p>

      <div className="table-wrapper">
        <table className="vercel-table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>Session ID</th>
              <th>Command</th>
              <th>Status</th>
              <th>Exit Code</th>
              <th>Duration (ms)</th>
            </tr>
          </thead>
          <tbody>
            {records.map(r => (
              <FragmentRow
                key={r.ID}
                r={r}
                nodes={nodes}
                onRerun={onRerun}
                expanded={expanded === r.ID}
                onToggle={() => setExpanded(expanded === r.ID ? null : r.ID)}
              />
            ))}
            {records.length === 0 && (
              <tr>
                <td colSpan={6} className="empty-state">No execution history found.</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {total > 0 && (
        <div className="pagination">
          <span className="pagination-info">
            Showing {firstOnPage}–{lastOnPage} of {total}
          </span>
          <button
            className="btn btn-secondary"
            disabled={offset === 0}
            onClick={() => goToPage(currentPage - 1)}
          >
            ← Prev
          </button>
          <span className="pagination-page">
            Page {currentPage} of {totalPages}
          </span>
          <button
            className="btn btn-secondary"
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => goToPage(currentPage + 1)}
          >
            Next →
          </button>
        </div>
      )}
    </div>
  )
}

const formatJson = (raw: string) => {
  if (!raw) return null
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function FragmentRow({ r, expanded, onToggle, nodes = [], onRerun }: {
  r: ExecutionRecord
  expanded: boolean
  onToggle: () => void
  nodes?: NodeInfo[]
  onRerun?: (nodeId: string, command: string) => void
}) {
  const [selectedNode, setSelectedNode] = useState<string>(() => nodes[0]?.id || '')

  useEffect(() => {
    if (!selectedNode && nodes.length > 0) {
      setSelectedNode(nodes[0].id)
    }
  }, [nodes, selectedNode])

  return (
    <>
      <tr className={expanded ? 'expanded' : ''} onClick={onToggle}>
        <td className="timestamp">{new Date(r.Timestamp).toLocaleString()}</td>
        <td className="mono">{r.SessionID || '-'}</td>
        <td className="mono command-cell">{r.Command}</td>
        <td>
          <span className={`status-badge ${r.Status}`}>
            {r.Status}
          </span>
        </td>
        <td className="mono">{r.ExitCode}</td>
        <td className="mono">{r.DurationMs}ms</td>
      </tr>
      {expanded && (
        <tr className="replay-row">
          <td colSpan={6}>
            <div className="replay-content">
              {onRerun && (
                <div className="replay-action-bar">
                  <div className="replay-action-info">
                    <span className="replay-action-label">Quick Re-run:</span>
                    <span className="mono replay-cmd-badge">{r.Command}</span>
                  </div>
                  <div className="replay-action-controls">
                    <select
                      className="rerun-select"
                      value={selectedNode}
                      onChange={(e) => setSelectedNode(e.target.value)}
                      disabled={nodes.length === 0}
                    >
                      {nodes.map(n => (
                        <option key={n.id} value={n.id}>
                          {n.hostname} ({n.id.slice(0, 10)}...)
                        </option>
                      ))}
                      {nodes.length === 0 && <option value="">No daemons online</option>}
                    </select>
                    <button
                      type="button"
                      className="btn-rerun"
                      disabled={nodes.length === 0 || !selectedNode}
                      onClick={(e) => {
                        e.stopPropagation()
                        onRerun(selectedNode, r.Command)
                      }}
                      title="Dispatch this command to the selected daemon and watch output live"
                    >
                      Re-run on Node ↻
                    </button>
                  </div>
                </div>
              )}
              <div className="replay-section">
                <h4>Request</h4>
                <pre>{formatJson(r.ReqJSON) || 'N/A'}</pre>
              </div>
              <div className="replay-section">
                <h4>Response</h4>
                <pre>{formatJson(r.RespJSON) || 'N/A'}</pre>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}