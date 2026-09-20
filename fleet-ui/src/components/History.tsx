import { useState, useEffect, useCallback, useMemo } from 'react'
import { DiffDrawer } from './DiffDrawer'

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

interface VerificationInfo {
  is_deterministic: boolean
  exit_code_match: boolean
  output_match: boolean
  diff_match: boolean
  similarity_score: number
  drift_summary: string
  replay_hash: string
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
                token={token}
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

function FragmentRow({ r, token, expanded, onToggle, nodes = [], onRerun }: {
  r: ExecutionRecord
  token: string
  expanded: boolean
  onToggle: () => void
  nodes?: NodeInfo[]
  onRerun?: (nodeId: string, command: string) => void
}) {
  const [selectedNode, setSelectedNode] = useState<string>(() => nodes[0]?.id || '')
  const [isDrawerOpen, setIsDrawerOpen] = useState<boolean>(false)
  const [drawerInitialFile, setDrawerInitialFile] = useState<string>('')

  // Rollback state
  const [isRollingBack, setIsRollingBack] = useState(false)
  const [rollbackStatus, setRollbackStatus] = useState<string | null>(null)

  // Verify state
  const [isVerifying, setIsVerifying] = useState(false)
  const [verifyResult, setVerifyResult] = useState<VerificationInfo | null>(null)

  const parsedResp = useMemo(() => {
    if (!r.RespJSON) return null
    try {
      return JSON.parse(r.RespJSON)
    } catch {
      return null
    }
  }, [r.RespJSON])

  const parsedReq = useMemo(() => {
    if (!r.ReqJSON) return null
    try {
      return JSON.parse(r.ReqJSON)
    } catch {
      return null
    }
  }, [r.ReqJSON])

  const filesChanged: string[] = parsedResp?.files_changed || []
  const fileDiffs: Record<string, string> = parsedResp?.file_diffs || {}
  const cwd: string = parsedResp?.cwd || parsedReq?.cwd || ''
  const rootCause = parsedResp?.error_root_cause
  const runHash: string = parsedResp?.run_hash || ''

  useEffect(() => {
    if (!selectedNode && nodes.length > 0) {
      setSelectedNode(nodes[0].id)
    }
  }, [nodes, selectedNode])

  const handleOpenDiff = (file: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setDrawerInitialFile(file)
    setIsDrawerOpen(true)
  }

  const handleRollback = async (id: number, e: React.MouseEvent) => {
    e.stopPropagation()
    setIsRollingBack(true)
    setRollbackStatus(null)
    try {
      const res = await fetch('/api/rollback', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {})
        },
        body: JSON.stringify({ id })
      })
      if (res.ok) {
        const data = await res.json()
        setRollbackStatus(`✓ ${data.message || 'Rolled back'}`)
      } else {
        const txt = await res.text()
        setRollbackStatus(`❌ Failed: ${txt}`)
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setRollbackStatus(`❌ Error: ${msg}`)
    } finally {
      setIsRollingBack(false)
    }
  }

  const handleVerify = async (id: number, e: React.MouseEvent) => {
    e.stopPropagation()
    setIsVerifying(true)
    setVerifyResult(null)
    try {
      const res = await fetch('/api/verify', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {})
        },
        body: JSON.stringify({ id })
      })
      if (res.ok) {
        const data = await res.json()
        setVerifyResult(data)
      }
    } catch (err) {
      console.error('Verification error', err)
    } finally {
      setIsVerifying(false)
    }
  }

  return (
    <>
      <tr className={expanded ? 'expanded' : ''} onClick={onToggle}>
        <td className="timestamp">{new Date(r.Timestamp).toLocaleString()}</td>
        <td className="mono">{r.SessionID || '-'}</td>
        <td className="mono command-cell">
          <div className="command-cell-content">
            <span className="command-text">{r.Command}</span>
            {filesChanged.length > 0 && (
              <span
                className="history-files-pill"
                title={`${filesChanged.length} files changed: ${filesChanged.join(', ')}`}
                onClick={(e) => handleOpenDiff(filesChanged[0], e)}
              >
                diff ({filesChanged.length})
              </span>
            )}
            {rootCause && (
              <span className="history-error-chip" title={`${rootCause.type}: ${rootCause.message}`}>
                ⚠️ {rootCause.type}
              </span>
            )}
          </div>
        </td>
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
              {/* Semantic Error Root Cause Card */}
              {rootCause && (
                <div className="root-cause-card">
                  <div className="root-cause-header">
                    <span className="root-cause-badge">
                      ⚠️ Root Cause: {rootCause.type}
                    </span>
                    {rootCause.file && (
                      <span className="root-cause-location mono">
                        {rootCause.file}
                        {rootCause.line ? `:${rootCause.line}` : ''}
                        {rootCause.column ? `:${rootCause.column}` : ''}
                      </span>
                    )}
                  </div>
                  <div className="root-cause-msg mono">{rootCause.message}</div>
                  {rootCause.snippet && (
                    <div className="root-cause-snippet mono">{rootCause.snippet}</div>
                  )}
                </div>
              )}

              {/* Cryptographic Verification Card */}
              {verifyResult && (
                <div className={`verify-card ${verifyResult.is_deterministic ? 'deterministic' : 'drift'}`}>
                  <div className="verify-header">
                    <span className="verify-status-badge">
                      {verifyResult.is_deterministic ? '✓ 100% REPRODUCIBLE (Deterministic)' : '⚠️ REPLAY DRIFT DETECTED'}
                    </span>
                    <span className="verify-score mono">
                      Similarity: {(verifyResult.similarity_score * 100).toFixed(1)}%
                    </span>
                  </div>
                  <div className="verify-summary mono">{verifyResult.drift_summary}</div>
                  {verifyResult.replay_hash && (
                    <div className="verify-hash mono">Replay Hash: {verifyResult.replay_hash}</div>
                  )}
                </div>
              )}

              {/* Action Bar with Quick Re-run and Cryptographic Verify */}
              <div className="replay-action-bar">
                <div className="replay-action-info">
                  <span className="replay-action-label">Actions:</span>
                  <span className="mono replay-cmd-badge">{r.Command}</span>
                  {runHash && (
                    <span className="mono run-hash-chip" title={`Fingerprint: ${runHash}`}>
                      🔐 {runHash.slice(0, 16)}...
                    </span>
                  )}
                </div>
                <div className="replay-action-controls">
                  <button
                    type="button"
                    className="btn-verify"
                    disabled={isVerifying}
                    onClick={(e) => handleVerify(r.ID, e)}
                    title="Cryptographically verify execution reproducibility"
                  >
                    {isVerifying ? 'Verifying...' : 'Verify ⛨'}
                  </button>

                  {onRerun && (
                    <>
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
                    </>
                  )}
                </div>
              </div>

              {/* Files Changed, Diff Inspector & Atomic Rollback Bar */}
              {filesChanged.length > 0 && (
                <div className="history-diff-bar">
                  <div className="history-diff-info">
                    <span className="diff-chip-title">
                      Files Changed ({filesChanged.length}):
                    </span>
                    <div className="history-diff-chips">
                      {filesChanged.map(f => (
                        <button
                          key={f}
                          type="button"
                          className="history-file-chip mono"
                          onClick={(e) => handleOpenDiff(f, e)}
                          title={`Click to inspect git diff for ${f}`}
                        >
                          📄 {f}
                        </button>
                      ))}
                    </div>
                  </div>
                  <div className="history-diff-actions">
                    {rollbackStatus && (
                      <span className="rollback-status-chip mono">{rollbackStatus}</span>
                    )}
                    <button
                      type="button"
                      className="btn-rollback"
                      disabled={isRollingBack}
                      onClick={(e) => handleRollback(r.ID, e)}
                      title="Surgically revert all files modified by this execution"
                    >
                      {isRollingBack ? 'Reverting...' : 'Undo Changes ↺'}
                    </button>
                    <button
                      type="button"
                      className="btn-diff-drawer"
                      onClick={(e) => handleOpenDiff(filesChanged[0], e)}
                    >
                      View Diff Drawer ⎘
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

      {/* Slide-over Diff Drawer */}
      {filesChanged.length > 0 && (
        <DiffDrawer
          isOpen={isDrawerOpen}
          onClose={() => setIsDrawerOpen(false)}
          files={filesChanged}
          diffs={fileDiffs}
          initialFile={drawerInitialFile}
          token={token}
          cwd={cwd}
        />
      )}
    </>
  )
}