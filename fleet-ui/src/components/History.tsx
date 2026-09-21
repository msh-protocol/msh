import { useState, useEffect, useCallback, useMemo, Fragment } from 'react'
import { DiffDrawer } from './DiffDrawer'
import { ExportModal } from './ExportModal'
import {
  TerminalIcon,
  FileTextIcon,
  SearchIcon,
  XIcon,
  CopyIcon,
  CheckIcon,
  AlertTriangleIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  ServerIcon,
  ChevronRightIcon,
  ArrowDownLeftIcon,
  ArrowUpRightIcon,
  DownloadIcon,
  Trash2Icon
} from './Icons'

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

interface ParsedExecutionRecord extends ExecutionRecord {
  parsedResp: any
  parsedReq: any
  filesChanged: string[]
  fileDiffs: Record<string, string>
  rootCause: any
  runHash: string
  cwd: string
}

const DEFAULT_PAGE_SIZE = 15

export interface NodeInfo {
  id: string
  hostname: string
  os: string
  arch: string
}

function formatRelativeTime(dateStr: string): string {
  try {
    const d = new Date(dateStr)
    const now = new Date()
    const diffSec = Math.floor((now.getTime() - d.getTime()) / 1000)
    if (diffSec < 10) return 'just now'
    if (diffSec < 60) return `${diffSec}s ago`
    const diffMin = Math.floor(diffSec / 60)
    if (diffMin < 60) return `${diffMin}m ago`
    const diffHour = Math.floor(diffMin / 60)
    if (diffHour < 24) return `${diffHour}h ago`
    const diffDay = Math.floor(diffHour / 24)
    if (diffDay < 7) return `${diffDay}d ago`
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  } catch {
    return dateStr
  }
}

function formatFullTime(dateStr: string): string {
  try {
    const d = new Date(dateStr)
    return d.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return dateStr
  }
}

export function History({ token, nodes = [], onRerun }: {
  token: string
  nodes?: NodeInfo[]
  onRerun?: (nodeId: string, command: string) => void
}) {
  const [records, setRecords] = useState<ExecutionRecord[]>([])
  const [offset, setOffset] = useState(0)
  const [pageSize] = useState(DEFAULT_PAGE_SIZE)
  const [total, setTotal] = useState(0)
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [diffRun, setDiffRun] = useState<ParsedExecutionRecord | null>(null)
  const [selectedFile, setSelectedFile] = useState<string>('')

  // Search & Filter state
  const [searchQuery, setSearchQuery] = useState('')
  const [filter, setFilter] = useState<'all' | 'success' | 'error' | 'diffs' | 'rootcause'>('all')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [copiedId, setCopiedId] = useState<number | null>(null)
  const [showExportModal, setShowExportModal] = useState(false)
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null)
  const [clearConfirm, setClearConfirm] = useState(false)

  const fetchHistory = useCallback(async (isManual = false) => {
    if (isManual) setIsRefreshing(true)
    try {
      const res = await fetch(`/api/history?limit=${pageSize}&offset=${offset}`, {
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
    } finally {
      if (isManual) {
        setTimeout(() => setIsRefreshing(false), 300)
      }
    }
  }, [token, offset, pageSize])

  useEffect(() => {
    fetchHistory()
    if (!autoRefresh) return
    const interval = setInterval(() => {
      if (offset === 0) fetchHistory()
    }, 4000)
    return () => clearInterval(interval)
  }, [fetchHistory, offset, autoRefresh])

  const parsedRecords: ParsedExecutionRecord[] = useMemo(() => {
    return records.map(r => {
      let parsedResp: any = null
      let parsedReq: any = null
      try { if (r.RespJSON) parsedResp = JSON.parse(r.RespJSON) } catch {}
      try { if (r.ReqJSON) parsedReq = JSON.parse(r.ReqJSON) } catch {}
      return {
        ...r,
        parsedResp,
        parsedReq,
        filesChanged: parsedResp?.files_changed || [],
        fileDiffs: parsedResp?.file_diffs || {},
        rootCause: parsedResp?.error_root_cause || null,
        runHash: parsedResp?.run_hash || '',
        cwd: parsedResp?.cwd || parsedReq?.cwd || '',
      }
    })
  }, [records])

  const stats = useMemo(() => {
    const successCount = records.filter(r => r.Status === 'success').length
    const errorCount = records.filter(r => r.Status !== 'success').length
    const diffsCount = parsedRecords.filter(r => r.filesChanged.length > 0).length
    const rootCauseCount = parsedRecords.filter(r => !!r.rootCause).length
    const totalDuration = records.reduce((acc, r) => acc + (r.DurationMs || 0), 0)
    const avgDuration = records.length > 0 ? Math.round(totalDuration / records.length) : 0
    const successRate = records.length > 0 ? Math.round((successCount / records.length) * 100) : 100

    return {
      successCount,
      errorCount,
      diffsCount,
      rootCauseCount,
      avgDuration,
      successRate
    }
  }, [records, parsedRecords])

  const filteredRecords = useMemo(() => {
    return parsedRecords.filter(r => {
      if (filter === 'success' && r.Status !== 'success') return false
      if (filter === 'error' && r.Status === 'success') return false
      if (filter === 'diffs' && r.filesChanged.length === 0) return false
      if (filter === 'rootcause' && !r.rootCause) return false

      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchCmd = r.Command.toLowerCase().includes(q)
        const matchSession = (r.SessionID || '').toLowerCase().includes(q)
        const matchHash = r.runHash.toLowerCase().includes(q)
        const matchFile = r.filesChanged.some((f: string) => f.toLowerCase().includes(q))
        const matchErr = r.rootCause ? `${r.rootCause.type} ${r.rootCause.message}`.toLowerCase().includes(q) : false
        if (!matchCmd && !matchSession && !matchHash && !matchFile && !matchErr) {
          return false
        }
      }
      return true
    })
  }, [parsedRecords, filter, searchQuery])

  const currentPage = Math.floor(offset / pageSize) + 1
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const firstOnPage = total === 0 ? 0 : offset + 1
  const lastOnPage = Math.min(offset + pageSize, total)

  const goToPage = (page: number) => {
    if (page < 1 || page > totalPages) return
    setOffset((page - 1) * pageSize)
  }

  // Clear confirmation states after 4 seconds
  useEffect(() => {
    if (confirmDeleteId !== null) {
      const t = setTimeout(() => setConfirmDeleteId(null), 4000)
      return () => clearTimeout(t)
    }
  }, [confirmDeleteId])

  useEffect(() => {
    if (clearConfirm) {
      const t = setTimeout(() => setClearConfirm(false), 4000)
      return () => clearTimeout(t)
    }
  }, [clearConfirm])

  const handleDeleteRecord = async (id: number) => {
    if (confirmDeleteId !== id) {
      setConfirmDeleteId(id)
      return
    }
    setConfirmDeleteId(null)
    setRecords(prev => prev.filter(r => r.ID !== id))
    setTotal(prev => Math.max(0, prev - 1))
    try {
      await fetch(`/api/history?id=${id}`, {
        method: 'DELETE',
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      })
    } catch (err) {
      console.error('Failed to delete history record', id, err)
    }
  }

  const handleClearHistory = async () => {
    if (!clearConfirm) {
      setClearConfirm(true)
      return
    }
    setClearConfirm(false)
    setRecords([])
    setTotal(0)
    try {
      await fetch(`/api/history?all=true`, {
        method: 'DELETE',
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      })
    } catch (err) {
      console.error('Failed to clear history', err)
    }
  }

  const copyCommand = (cmd: string, id: number, e: React.MouseEvent) => {
    e.stopPropagation()
    navigator.clipboard.writeText(cmd)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 1800)
  }

  return (
    <div className="ledger-root">
      {/* Sleek Compact Telemetry Bar */}
      <div className="ledger-telemetry-bar">
        <div className="telemetry-item">
          <div className="telemetry-label">TOTAL RUNS</div>
          <div className="telemetry-metric">
            <span className="telemetry-num mono">{total}</span>
            <span className="telemetry-hint">commands recorded</span>
          </div>
        </div>

        <div className="telemetry-divider"></div>

        <div className="telemetry-item">
          <div className="telemetry-label">SUCCESS RATE</div>
          <div className="telemetry-metric">
            <span className={`telemetry-num mono ${stats.successRate >= 90 ? 'text-sage' : 'text-terracotta'}`}>
              {stats.successRate}%
            </span>
            <span className="telemetry-badge sage">
              {stats.successCount} OK · {stats.errorCount} ERR
            </span>
          </div>
        </div>

        <div className="telemetry-divider"></div>

        <div className="telemetry-item">
          <div className="telemetry-label">AVG LATENCY</div>
          <div className="telemetry-metric">
            <span className="telemetry-num mono text-terracotta">
              {stats.avgDuration < 1000 ? `${stats.avgDuration}ms` : `${(stats.avgDuration / 1000).toFixed(2)}s`}
            </span>
            <span className="telemetry-hint">PTY duration</span>
          </div>
        </div>

        <div className="telemetry-divider"></div>

        <div className="telemetry-item">
          <div className="telemetry-label">MUTATIONS</div>
          <div className="telemetry-metric">
            <span className="telemetry-num mono">{stats.diffsCount}</span>
            <span className="telemetry-hint">with file diffs</span>
          </div>
        </div>

        <div className="telemetry-actions-right">
          <button
            type="button"
            className={`btn-stream-toggle ${autoRefresh ? 'active' : ''}`}
            onClick={() => setAutoRefresh(!autoRefresh)}
            title={autoRefresh ? 'Streaming live - click to pause' : 'Stream paused - click to resume'}
          >
            <span className={`status-dot ${autoRefresh ? 'pulse-green' : 'gray'}`}></span>
            <span>{autoRefresh ? 'Live' : 'Paused'}</span>
          </button>
          <button
            type="button"
            className={`btn-refresh-telemetry ${isRefreshing ? 'spinning' : ''}`}
            onClick={() => fetchHistory(true)}
            disabled={isRefreshing}
            title="Refresh history"
          >
            <RefreshCwIcon size={12} className={isRefreshing ? 'spinning' : ''} />
          </button>
        </div>
      </div>

      {/* Main Ledger Card Container */}
      <div className="ledger-card">
        {/* Table Filter & Search Header */}
        <div className="ledger-header">
          <div className="ledger-search-box">
            <SearchIcon size={13} className="search-icon" />
            <input
              type="text"
              className="ledger-search-input"
              placeholder="Filter by command, session, hash, or path..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
            {searchQuery && (
              <button
                type="button"
                className="btn-clear"
                onClick={() => setSearchQuery('')}
              >
                <XIcon size={11} />
              </button>
            )}
          </div>

          <div className="ledger-filter-group">
            <button
              type="button"
              className={`filter-pill ${filter === 'all' ? 'active' : ''}`}
              onClick={() => setFilter('all')}
            >
              All <span className="pill-count mono">{records.length}</span>
            </button>
            <button
              type="button"
              className={`filter-pill ${filter === 'success' ? 'active' : ''}`}
              onClick={() => setFilter('success')}
            >
              <span className="dot dot-sage"></span> Succeeded <span className="pill-count mono">{stats.successCount}</span>
            </button>
            <button
              type="button"
              className={`filter-pill ${filter === 'error' ? 'active' : ''}`}
              onClick={() => setFilter('error')}
            >
              <span className="dot dot-terracotta"></span> Failed <span className="pill-count mono">{stats.errorCount}</span>
            </button>
            <button
              type="button"
              className={`filter-pill ${filter === 'diffs' ? 'active' : ''}`}
              onClick={() => setFilter('diffs')}
            >
              <FileTextIcon size={11} /> Diffs <span className="pill-count mono">{stats.diffsCount}</span>
            </button>
            {stats.rootCauseCount > 0 && (
              <button
                type="button"
                className={`filter-pill ${filter === 'rootcause' ? 'active' : ''}`}
                onClick={() => setFilter('rootcause')}
              >
                <AlertTriangleIcon size={11} /> Root Cause <span className="pill-count mono">{stats.rootCauseCount}</span>
              </button>
            )}
          </div>

          {/* Export & Clear History Actions */}
          <div className="ledger-header-actions">
            <button
              type="button"
              className={`btn-clear-history ${clearConfirm ? 'confirming' : ''}`}
              onClick={handleClearHistory}
              disabled={records.length === 0}
              title={clearConfirm ? 'Click again to permanently wipe all history' : 'Clear all execution history'}
            >
              <Trash2Icon size={12} />
              <span>{clearConfirm ? 'Confirm Clear All?' : 'Clear History'}</span>
            </button>

            <button
              type="button"
              className="btn-export-history"
              onClick={() => setShowExportModal(true)}
              title="Choose format, scope, and destination to export history"
            >
              <DownloadIcon size={12} />
              <span>Export History</span>
            </button>
          </div>
        </div>

        {/* High-Density Scannable Data Table */}
        <div className="ledger-table-container">
          <table className="ledger-table">
            <thead>
              <tr>
                <th className="col-chevron"></th>
                <th className="col-status">Status</th>
                <th className="col-command">Command Execution</th>
                <th className="col-origin">Origin / Node</th>
                <th className="col-duration">Duration</th>
                <th className="col-diffs">Diffs</th>
                <th className="col-time">Time</th>
                <th className="col-actions"></th>
              </tr>
            </thead>
            <tbody>
              {filteredRecords.map((r) => {
                const isExpanded = expandedId === r.ID
                const isSuccess = r.Status === 'success'
                const hasDiffs = r.filesChanged.length > 0

                return (
                  <Fragment key={r.ID}>
                    <tr
                      className={`ledger-row ${isExpanded ? 'row-expanded' : ''} ${!isSuccess ? 'row-error' : ''}`}
                      onClick={() => setExpandedId(isExpanded ? null : r.ID)}
                    >
                      <td className="cell-chevron">
                        <div className={`chevron-icon ${isExpanded ? 'rotated' : ''}`}>
                          <ChevronRightIcon size={13} />
                        </div>
                      </td>

                      <td className="cell-status">
                        <span className={`status-tag ${isSuccess ? 'tag-success' : 'tag-error'}`}>
                          {isSuccess ? 'exit 0' : `exit ${r.ExitCode}`}
                        </span>
                      </td>

                      <td className="cell-command">
                        <div className="command-display">
                          <span className="prompt-sym">$</span>
                          <span className="command-str mono" title={r.Command}>
                            {r.Command}
                          </span>
                        </div>
                      </td>

                      <td className="cell-origin">
                        <span className="origin-text mono">
                          {r.SessionID.startsWith('local-') ? (
                            <span className="badge-local"><TerminalIcon size={10} /> local</span>
                          ) : (
                            <span className="badge-remote"><ServerIcon size={10} /> {r.SessionID.slice(0, 10)}</span>
                          )}
                        </span>
                      </td>

                      <td className="cell-duration">
                        <span className="duration-text mono">
                          {r.DurationMs < 1000 ? `${r.DurationMs}ms` : `${(r.DurationMs / 1000).toFixed(2)}s`}
                        </span>
                      </td>

                      <td className="cell-diffs">
                        {hasDiffs ? (
                          <button
                            type="button"
                            className="diff-tag-btn"
                            onClick={(e) => {
                              e.stopPropagation()
                              setSelectedFile(r.filesChanged[0] || '')
                              setDiffRun(r)
                            }}
                            title="Open workspace file diff"
                          >
                            <FileTextIcon size={10} />
                            <span>{r.filesChanged.length} file{r.filesChanged.length > 1 ? 's' : ''}</span>
                          </button>
                        ) : (
                          <span className="no-diffs">—</span>
                        )}
                      </td>

                      <td className="cell-time" title={formatFullTime(r.Timestamp)}>
                        <span className="time-text">{formatRelativeTime(r.Timestamp)}</span>
                      </td>

                      <td className="cell-actions" onClick={(e) => e.stopPropagation()}>
                        <div className="action-btn-group">
                          <button
                            type="button"
                            className={`btn-action-icon ${copiedId === r.ID ? 'copied' : ''}`}
                            onClick={(e) => copyCommand(r.Command, r.ID, e)}
                            title="Copy command"
                          >
                            {copiedId === r.ID ? <CheckIcon size={12} /> : <CopyIcon size={12} />}
                          </button>
                          {onRerun && nodes.length > 0 && (
                            <button
                              type="button"
                              className="btn-action-icon btn-rerun"
                              onClick={() => onRerun(nodes[0].id, r.Command)}
                              title="Restart / Re-run command on active worker"
                            >
                              <RotateCcwIcon size={12} />
                            </button>
                          )}
                          <button
                            type="button"
                            className={`btn-action-icon btn-action-delete ${confirmDeleteId === r.ID ? 'confirming' : ''}`}
                            onClick={(e) => {
                              e.stopPropagation()
                              handleDeleteRecord(r.ID)
                            }}
                            title={confirmDeleteId === r.ID ? 'Click again to permanently delete' : 'Delete from history'}
                          >
                            <Trash2Icon size={12} />
                          </button>
                        </div>
                      </td>
                    </tr>

                    {/* In-Place Expanded Execution Record Drawer / Docket */}
                    {expandedId === r.ID && (
                      <tr className="ledger-expanded-row">
                        <td colSpan={8} className="expanded-td" onClick={(e) => e.stopPropagation()}>
                          <div className="ledger-expanded-inspector">
                            {/* Error Root Cause Alert if any */}
                            {r.rootCause && (
                              <div className="inspector-alert error">
                                <div className="alert-head">
                                  <AlertTriangleIcon size={13} />
                                  <span className="alert-type mono">{r.rootCause.type}</span>
                                </div>
                                <div className="alert-msg">{r.rootCause.message}</div>
                              </div>
                            )}

                            {/* Meta Grid */}
                            <div className="inspector-meta-strip">
                              <div className="meta-cell">
                                <span className="meta-k">RUN HASH</span>
                                <span className="meta-v mono">{r.runHash ? r.runHash.slice(0, 16) : '—'}</span>
                              </div>
                              <div className="meta-cell">
                                <span className="meta-k">WORKING DIRECTORY</span>
                                <span className="meta-v mono">{r.cwd || '~'}</span>
                              </div>
                              <div className="meta-cell">
                                <span className="meta-k">SESSION ID</span>
                                <span className="meta-v mono">
                                  {r.SessionID || r.parsedResp?.session_id || r.parsedReq?.session_id || 'local-exec'}
                                </span>
                              </div>
                              <div className="meta-cell">
                                <span className="meta-k">TIMESTAMP</span>
                                <span className="meta-v mono">{formatFullTime(r.Timestamp)}</span>
                              </div>
                              {r.filesChanged.length > 0 && (
                                <div className="meta-cell meta-cell-files">
                                  <span className="meta-k">CHANGED FILES</span>
                                  <div className="meta-file-chips-wrap">
                                    {r.filesChanged.map((file) => (
                                      <button
                                        key={file}
                                        type="button"
                                        className="meta-file-chip"
                                        onClick={() => {
                                          setSelectedFile(file)
                                          setDiffRun(r)
                                        }}
                                        title={`Inspect diff for ${file}`}
                                      >
                                        <FileTextIcon size={11} />
                                        <span className="mono file-name">{file}</span>
                                      </button>
                                    ))}
                                    <button
                                      type="button"
                                      className="btn-inspect-diff-direct"
                                      onClick={() => {
                                        setSelectedFile(r.filesChanged[0] || '')
                                        setDiffRun(r)
                                      }}
                                    >
                                      <span>All Diffs ({r.filesChanged.length})</span>
                                    </button>
                                  </div>
                                </div>
                              )}
                            </div>

                            {/* JSON Payloads Side by Side */}
                            <div className="inspector-json-grid" style={{ gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', width: '100%' }}>
                              <div className="json-box" style={{ minWidth: 0, maxWidth: '100%', overflow: 'hidden' }}>
                                <div className="json-box-head">
                                  <div className="json-title">
                                    <ArrowDownLeftIcon size={12} />
                                    <span>ExecRequest</span>
                                  </div>
                                  <button
                                    type="button"
                                    className="btn-copy-json"
                                    onClick={() => navigator.clipboard.writeText(r.ReqJSON || '{}')}
                                  >
                                    <CopyIcon size={11} /> Copy
                                  </button>
                                </div>
                                <pre
                                  className="json-pre mono"
                                  style={{
                                    whiteSpace: 'pre-wrap',
                                    wordBreak: 'break-all',
                                    overflowWrap: 'anywhere',
                                    overflowX: 'hidden',
                                    maxWidth: '100%'
                                  }}
                                >
                                  <code
                                    style={{
                                      whiteSpace: 'pre-wrap',
                                      wordBreak: 'break-all',
                                      overflowWrap: 'anywhere',
                                      display: 'block',
                                      maxWidth: '100%'
                                    }}
                                  >
                                    {r.ReqJSON ? (
                                      (() => {
                                        try { return JSON.stringify(JSON.parse(r.ReqJSON), null, 2) }
                                        catch { return r.ReqJSON }
                                      })()
                                    ) : '// None'}
                                  </code>
                                </pre>
                              </div>

                              <div className="json-box" style={{ minWidth: 0, maxWidth: '100%', overflow: 'hidden' }}>
                                <div className="json-box-head">
                                  <div className="json-title">
                                    <ArrowUpRightIcon size={12} />
                                    <span>ExecResponse</span>
                                  </div>
                                  <button
                                    type="button"
                                    className="btn-copy-json"
                                    onClick={() => navigator.clipboard.writeText(r.RespJSON || '{}')}
                                  >
                                    <CopyIcon size={11} /> Copy
                                  </button>
                                </div>
                                <pre
                                  className="json-pre mono"
                                  style={{
                                    whiteSpace: 'pre-wrap',
                                    wordBreak: 'break-all',
                                    overflowWrap: 'anywhere',
                                    overflowX: 'hidden',
                                    maxWidth: '100%'
                                  }}
                                >
                                  <code
                                    style={{
                                      whiteSpace: 'pre-wrap',
                                      wordBreak: 'break-all',
                                      overflowWrap: 'anywhere',
                                      display: 'block',
                                      maxWidth: '100%'
                                    }}
                                  >
                                    {r.RespJSON ? (
                                      (() => {
                                        try { return JSON.stringify(JSON.parse(r.RespJSON), null, 2) }
                                        catch { return r.RespJSON }
                                      })()
                                    ) : '// None'}
                                  </code>
                                </pre>
                              </div>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}

              {/* Empty Results State */}
              {filteredRecords.length === 0 && (
                <tr>
                  <td colSpan={8} className="empty-ledger-td">
                    <div className="empty-ledger-state">
                      <TerminalIcon size={28} className="empty-icon" />
                      <div className="empty-heading">No execution records found</div>
                      <div className="empty-sub">
                        {searchQuery ? `No results match query "${searchQuery}"` : 'Execute commands via CLI or Swarm Broadcast to view telemetry.'}
                      </div>
                      {searchQuery && (
                        <button
                          type="button"
                          className="btn-reset-search"
                          onClick={() => {
                            setSearchQuery('')
                            setFilter('all')
                          }}
                        >
                          Reset Filters
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Compact Pagination Bar */}
        {total > 0 && (
          <div className="ledger-pagination">
            <div className="page-summary">
              Showing <span className="mono font-bold">{firstOnPage}</span>–<span className="mono font-bold">{lastOnPage}</span> of{' '}
              <span className="mono font-bold">{total}</span>
            </div>

            <div className="page-btns">
              <button
                type="button"
                className="btn-nav"
                disabled={currentPage <= 1}
                onClick={() => goToPage(currentPage - 1)}
              >
                Prev
              </button>
              <span className="page-current mono">
                {currentPage} / {totalPages}
              </span>
              <button
                type="button"
                className="btn-nav"
                disabled={currentPage >= totalPages}
                onClick={() => goToPage(currentPage + 1)}
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Visual File Diff Drawer Modal */}
      {diffRun && (
        <DiffDrawer
          isOpen={!!diffRun}
          onClose={() => {
            setDiffRun(null)
            setSelectedFile('')
          }}
          files={diffRun.filesChanged}
          diffs={diffRun.fileDiffs}
          initialFile={selectedFile}
          cwd={diffRun.cwd}
          token={token}
          runId={diffRun.ID}
          onRollbackSuccess={() => fetchHistory()}
        />
      )}

      {/* Export & Download Chooser Modal */}
      <ExportModal
        isOpen={showExportModal}
        onClose={() => setShowExportModal(false)}
        records={filteredRecords}
        totalCount={total}
        token={token}
      />
    </div>
  )
}