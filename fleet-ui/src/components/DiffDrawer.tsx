import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  FileTextIcon,
  CopyIcon,
  CheckIcon,
  XIcon,
  ServerIcon,
  RotateCcwIcon,
  ColumnsIcon,
  RowsIcon
} from './Icons'
import './DiffDrawer.css'

interface DiffDrawerProps {
  isOpen: boolean
  onClose: () => void
  files: string[]
  diffs?: Record<string, string>
  initialFile?: string
  token?: string
  cwd?: string
  runId?: number
  onRollbackSuccess?: (revertedFile?: string) => void
}

interface ParsedDiffLine {
  type: 'hunk' | 'add' | 'del' | 'context' | 'meta'
  oldNum?: number
  newNum?: number
  content: string
}

interface SplitDiffRow {
  type: 'hunk' | 'meta' | 'pair'
  hunkContent?: string
  metaContent?: string
  left?: {
    num?: number
    content: string
    type: 'del' | 'context'
  }
  right?: {
    num?: number
    content: string
    type: 'add' | 'context'
  }
}

export function DiffDrawer({
  isOpen,
  onClose,
  files = [],
  diffs = {},
  initialFile,
  token,
  cwd,
  runId,
  onRollbackSuccess
}: DiffDrawerProps) {
  const [activeFile, setActiveFile] = useState<string>(() => initialFile || files[0] || '')
  const [fileDiffMap, setFileDiffMap] = useState<Record<string, string>>(() => {
    const initial: Record<string, string> = {}
    if (diffs) {
      for (const [k, v] of Object.entries(diffs)) {
        initial[k] = v
        initial[k.replace(/\\/g, '/')] = v
        initial[k.replace(/\//g, '\\')] = v
      }
    }
    return initial
  })
  const [loading, setLoading] = useState<boolean>(false)
  const [copied, setCopied] = useState<boolean>(false)

  // View mode: 'unified' vs 'split'
  const [viewMode, setViewMode] = useState<'unified' | 'split'>(() => {
    try {
      return (localStorage.getItem('msh_diff_view_mode') as 'unified' | 'split') || 'unified'
    } catch {
      return 'unified'
    }
  })

  const handleSetViewMode = (mode: 'unified' | 'split') => {
    setViewMode(mode)
    try {
      localStorage.setItem('msh_diff_view_mode', mode)
    } catch {}
  }

  // Rollback state
  const [revertConfirm, setRevertConfirm] = useState<boolean>(false)
  const [revertAllConfirm, setRevertAllConfirm] = useState<boolean>(false)
  const [isReverting, setIsReverting] = useState<boolean>(false)
  const [revertedFiles, setRevertedFiles] = useState<Set<string>>(new Set())
  const [revertMessage, setRevertMessage] = useState<string>('')
  const [revertError, setRevertError] = useState<string>('')

  useEffect(() => {
    if (revertConfirm) {
      const timer = setTimeout(() => setRevertConfirm(false), 4500)
      return () => clearTimeout(timer)
    }
  }, [revertConfirm])

  useEffect(() => {
    if (revertAllConfirm) {
      const timer = setTimeout(() => setRevertAllConfirm(false), 4500)
      return () => clearTimeout(timer)
    }
  }, [revertAllConfirm])

  useEffect(() => {
    if (revertError) {
      const timer = setTimeout(() => setRevertError(''), 7000)
      return () => clearTimeout(timer)
    }
  }, [revertError])

  // Reset transient confirmation and errors when active file changes
  useEffect(() => {
    setRevertConfirm(false)
    setRevertError('')
  }, [activeFile])

  // Sync incoming diffs with path normalization
  useEffect(() => {
    if (diffs && Object.keys(diffs).length > 0) {
      setFileDiffMap(prev => {
        const next = { ...prev }
        for (const [k, v] of Object.entries(diffs)) {
          next[k] = v
          next[k.replace(/\\/g, '/')] = v
          next[k.replace(/\//g, '\\')] = v
        }
        return next
      })
    }
  }, [diffs])

  // Set active file when initialFile prop changes from parent
  useEffect(() => {
    if (initialFile) {
      setActiveFile(initialFile)
    } else if (files.length > 0) {
      setActiveFile(files[0])
    }
  }, [initialFile, files])

  // Fallback if active file is not present in files list
  useEffect(() => {
    if (files.length > 0 && (!activeFile || !files.some(f => f === activeFile || f.replace(/\\/g, '/') === activeFile.replace(/\\/g, '/')))) {
      setActiveFile(files[0])
    }
  }, [files, activeFile])

  // Fetch diff on-demand if not already present in fileDiffMap
  const fetchMissingDiff = useCallback(async (file: string) => {
    if (!file) return
    const norm = file.replace(/\\/g, '/')
    const back = file.replace(/\//g, '\\')

    setLoading(true)
    try {
      const q = new URLSearchParams({ file: norm })
      if (cwd) q.set('cwd', cwd)
      const res = await fetch(`/api/diff?${q.toString()}`, {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      })
      if (res.ok) {
        const data = await res.json()
        const text = data.diff ?? ''
        setFileDiffMap(prev => ({
          ...prev,
          [file]: text,
          [norm]: text,
          [back]: text
        }))
      } else {
        setFileDiffMap(prev => ({
          ...prev,
          [file]: '',
          [norm]: '',
          [back]: ''
        }))
      }
    } catch (err) {
      console.error('Failed to fetch diff for file', file, err)
      setFileDiffMap(prev => ({
        ...prev,
        [file]: '',
        [norm]: '',
        [back]: ''
      }))
    } finally {
      setLoading(false)
    }
  }, [cwd, token])

  useEffect(() => {
    if (isOpen && activeFile) {
      const norm = activeFile.replace(/\\/g, '/')
      const back = activeFile.replace(/\//g, '\\')
      const isKnown =
        fileDiffMap[activeFile] !== undefined ||
        fileDiffMap[norm] !== undefined ||
        fileDiffMap[back] !== undefined

      if (!isKnown) {
        fetchMissingDiff(activeFile)
      }
    }
  }, [isOpen, activeFile, fileDiffMap, fetchMissingDiff])

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

  const currentDiff = useMemo(() => {
    if (!activeFile) return ''
    if (fileDiffMap[activeFile] !== undefined) return fileDiffMap[activeFile]
    const slash = activeFile.replace(/\\/g, '/')
    if (fileDiffMap[slash] !== undefined) return fileDiffMap[slash]
    const backslash = activeFile.replace(/\//g, '\\')
    if (fileDiffMap[backslash] !== undefined) return fileDiffMap[backslash]
    return ''
  }, [fileDiffMap, activeFile])

  // Intelligent binary asset detection
  const isBinary = useMemo(() => {
    if (!currentDiff && !activeFile) return false
    const lower = currentDiff.toLowerCase()
    return (
      lower.includes('binary file') ||
      lower.includes('binary files') ||
      /\.(exe|dll|so|dylib|bin|png|jpg|jpeg|gif|ico|webp|pdf|zip|tar|gz|wasm|lock)$/i.test(activeFile)
    )
  }, [currentDiff, activeFile])

  // Parse diff lines for unified viewer
  const parsedLines = useMemo(() => {
    if (!currentDiff || isBinary) return []
    const rawLines = currentDiff.split('\n')
    const result: ParsedDiffLine[] = []
    let oldNum = 0
    let newNum = 0
    let hasSeenHunk = false

    for (let i = 0; i < rawLines.length; i++) {
      const rawLine = rawLines[i]
      const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine

      if (!line) continue

      if (line.startsWith('@@')) {
        hasSeenHunk = true
        const match = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/)
        if (match) {
          oldNum = parseInt(match[1], 10)
          newNum = parseInt(match[2], 10)
        }
        result.push({ type: 'hunk', content: line })
      } else if (
        !hasSeenHunk ||
        line.startsWith('diff --git') ||
        line.startsWith('index ') ||
        line.startsWith('--- ') ||
        line.startsWith('+++ ') ||
        line.startsWith('new file mode ') ||
        line.startsWith('deleted file mode ') ||
        line.startsWith('old mode ') ||
        line.startsWith('similarity index ') ||
        line.startsWith('\\')
      ) {
        result.push({ type: 'meta', content: line })
      } else if (line.startsWith('+')) {
        result.push({ type: 'add', newNum: newNum++, content: line.slice(1) })
      } else if (line.startsWith('-')) {
        result.push({ type: 'del', oldNum: oldNum++, content: line.slice(1) })
      } else if (line.startsWith(' ')) {
        result.push({ type: 'context', oldNum: oldNum++, newNum: newNum++, content: line.slice(1) })
      } else {
        result.push({ type: 'meta', content: line })
      }
    }
    return result
  }, [currentDiff, isBinary])

  // Pair lines for side-by-side split viewer
  const splitRows = useMemo(() => {
    if (!currentDiff || isBinary) return []
    const rows: SplitDiffRow[] = []

    let delBatch: ParsedDiffLine[] = []
    let addBatch: ParsedDiffLine[] = []

    const flushBatches = () => {
      const maxLen = Math.max(delBatch.length, addBatch.length)
      for (let i = 0; i < maxLen; i++) {
        const d = delBatch[i]
        const a = addBatch[i]
        rows.push({
          type: 'pair',
          left: d ? { num: d.oldNum, content: d.content, type: 'del' } : undefined,
          right: a ? { num: a.newNum, content: a.content, type: 'add' } : undefined,
        })
      }
      delBatch = []
      addBatch = []
    }

    for (const line of parsedLines) {
      if (line.type === 'del') {
        delBatch.push(line)
      } else if (line.type === 'add') {
        addBatch.push(line)
      } else {
        flushBatches()
        if (line.type === 'context') {
          rows.push({
            type: 'pair',
            left: { num: line.oldNum, content: line.content, type: 'context' },
            right: { num: line.newNum, content: line.content, type: 'context' },
          })
        } else if (line.type === 'hunk') {
          rows.push({ type: 'hunk', hunkContent: line.content })
        } else if (line.type === 'meta') {
          rows.push({ type: 'meta', metaContent: line.content })
        }
      }
    }
    flushBatches()
    return rows
  }, [parsedLines, currentDiff, isBinary])

  // Calculate additions & deletions
  const stats = useMemo(() => {
    let adds = 0
    let dels = 0
    parsedLines.forEach(l => {
      if (l.type === 'add') adds++
      if (l.type === 'del') dels++
    })
    return { adds, dels }
  }, [parsedLines])

  const copyDiff = () => {
    if (!currentDiff) return
    navigator.clipboard.writeText(currentDiff)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Single-File Surgical Rollback
  const handleRevertFile = async () => {
    if (!activeFile || !runId) return
    if (!revertConfirm) {
      setRevertConfirm(true)
      return
    }

    setIsReverting(true)
    setRevertConfirm(false)
    setRevertError('')
    try {
      const res = await fetch('/api/rollback', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {})
        },
        body: JSON.stringify({ id: runId, file: activeFile })
      })

      if (res.ok) {
        setRevertedFiles(prev => new Set(prev).add(activeFile))
        setRevertMessage(`✓ Successfully reverted ${activeFile}`)
        setRevertError('')
        setTimeout(() => setRevertMessage(''), 4500)
        onRollbackSuccess?.(activeFile)
      } else {
        const errText = await res.text()
        const cleanErr = errText.replace(/^Rollback failed:\s*/i, '').trim()
        setRevertError(cleanErr || `Unable to rollback ${activeFile}`)
      }
    } catch (err: any) {
      setRevertError(err.message || String(err))
    } finally {
      setIsReverting(false)
    }
  }

  // Revert All Files in Run
  const handleRevertAll = async () => {
    if (!runId) return
    if (!revertAllConfirm) {
      setRevertAllConfirm(true)
      return
    }

    setIsReverting(true)
    setRevertAllConfirm(false)
    setRevertError('')
    try {
      const res = await fetch('/api/rollback', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {})
        },
        body: JSON.stringify({ id: runId })
      })

      if (res.ok) {
        files.forEach(f => setRevertedFiles(prev => new Set(prev).add(f)))
        setRevertMessage(`✓ Reverted all ${files.length} file(s)`)
        setRevertError('')
        setTimeout(() => setRevertMessage(''), 4500)
        onRollbackSuccess?.()
      } else {
        const errText = await res.text()
        const cleanErr = errText.replace(/^Rollback failed:\s*/i, '').trim()
        setRevertError(cleanErr || 'Unable to rollback execution changes')
      }
    } catch (err: any) {
      setRevertError(err.message || String(err))
    } finally {
      setIsReverting(false)
    }
  }

  if (!isOpen) return null

  const isFileReverted = revertedFiles.has(activeFile)

  return (
    <div className="diff-drawer-backdrop" onClick={onClose}>
      <div
        className={`diff-drawer ${viewMode === 'split' ? 'diff-drawer-split' : ''}`}
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="diff-drawer-header">
          <div className="diff-header-left">
            <span className="diff-icon">
              <FileTextIcon size={16} />
            </span>
            <div>
              <h3 className="diff-title">File Revisions & Diffs</h3>
              <span className="diff-subtitle">
                {files.length} {files.length === 1 ? 'file' : 'files'} recorded in execution {runId ? `#${runId}` : ''}
              </span>
            </div>
            {(stats.adds > 0 || stats.dels > 0) && (
              <div className="diff-stats-pill">
                <span className="diff-add-stat">+{stats.adds}</span>
                <span className="diff-del-stat">-{stats.dels}</span>
              </div>
            )}
          </div>

          <div className="diff-header-right">
            {runId && files.length > 1 && (
              <button
                type="button"
                className={`btn-drawer-rollback-all ${revertAllConfirm ? 'confirming' : ''}`}
                onClick={handleRevertAll}
                disabled={isReverting}
                title="Undo changes across all files in this execution"
              >
                <RotateCcwIcon size={12} className={isReverting ? 'spin-ccw' : ''} />
                <span>{revertAllConfirm ? 'Confirm Undo All?' : 'Undo Run Changes'}</span>
              </button>
            )}

            <button
              type="button"
              className="btn-drawer-action"
              onClick={copyDiff}
              disabled={!currentDiff || isBinary}
              title="Copy unified diff to clipboard"
            >
              {copied ? <CheckIcon size={12} /> : <CopyIcon size={12} />}
              <span>{copied ? 'Copied' : 'Copy Diff'}</span>
            </button>

            <button
              type="button"
              className="diff-close-btn"
              onClick={onClose}
              title="Close drawer (Esc)"
            >
              <XIcon size={14} />
              <span className="kbd-shortcut mono">ESC</span>
            </button>
          </div>
        </div>

        {/* In-Drawer Status & Error Banners */}
        {revertMessage && (
          <div className="diff-banner diff-banner-success mono">
            <span className="diff-banner-icon">✓</span>
            <span className="diff-banner-text">{revertMessage}</span>
            <button
              type="button"
              className="diff-banner-close"
              onClick={() => setRevertMessage('')}
              title="Dismiss"
            >
              ✕
            </button>
          </div>
        )}
        {revertError && (
          <div className="diff-banner diff-banner-error mono">
            <span className="diff-banner-icon">⚠️</span>
            <span className="diff-banner-text">{revertError}</span>
            <button
              type="button"
              className="diff-banner-close"
              onClick={() => setRevertError('')}
              title="Dismiss"
            >
              ✕
            </button>
          </div>
        )}

        {/* File Navigator Tabs */}
        {files.length > 0 && (
          <div
            className="diff-file-tabs"
            onWheel={(e) => {
              if (e.deltaY !== 0) {
                e.currentTarget.scrollLeft += e.deltaY
              }
            }}
          >
            {files.map(f => {
              const normF = f.replace(/\\/g, '/')
              const normActive = activeFile.replace(/\\/g, '/')
              const isActive = activeFile === f || normActive === normF
              const diffText = fileDiffMap[f] || fileDiffMap[normF] || ''
              const isDeleted = diffText.includes('+++ /dev/null')
              const isAdded = diffText.includes('--- /dev/null')
              const hasBeenReverted = revertedFiles.has(f)
              let badgeType = 'M'
              if (isAdded) badgeType = 'A'
              if (isDeleted) badgeType = 'D'

              return (
                <button
                  type="button"
                  key={f}
                  className={`diff-tab-btn ${isActive ? 'active' : ''} ${hasBeenReverted ? 'tab-reverted' : ''}`}
                  onClick={() => setActiveFile(f)}
                  title={f}
                >
                  <span className={`file-badge file-badge-${badgeType.toLowerCase()}`}>
                    {badgeType}
                  </span>
                  <span className="diff-tab-name mono">{f}</span>
                  {hasBeenReverted && (
                    <span className="tab-reverted-badge mono">Reverted</span>
                  )}
                </button>
              )
            })}
          </div>
        )}

        {/* Diff Content Viewer */}
        <div className="diff-body">
          {loading ? (
            <div className="diff-empty-state">
              <div className="spinner"></div>
              <span>Computing unified diff...</span>
            </div>
          ) : isBinary ? (
            /* Dedicated High-Craft Binary Asset Specimen Card */
            <div className="diff-binary-wrapper">
              <div className="diff-binary-card">
                <div className="diff-binary-icon-wrap">
                  <ServerIcon size={32} />
                </div>
                <div className="diff-binary-details">
                  <div className="diff-binary-tag mono">
                    {activeFile.split('.').pop()?.toUpperCase() || 'BIN'} ASSET
                  </div>
                  <h4 className="diff-binary-title">Binary Asset Modified</h4>
                  <p className="diff-binary-desc">
                    Binary or compiled payload <code>{activeFile}</code> was altered during command execution. Textual line-by-line unified diff is not applicable.
                  </p>
                  <div className="diff-binary-meta mono">
                    <span className="meta-label">Path:</span>
                    <span className="meta-val">{activeFile}</span>
                  </div>

                  {/* Informational Callout regarding Rollback on Binaries */}
                  <div className="diff-binary-notice mono">
                    <span className="binary-notice-icon">ℹ</span>
                    <span>Rollback is unsupported for compiled executables and binary payloads (non-textual machine code cannot be reverse-patched).</span>
                  </div>

                  {runId && (
                    <div className="diff-binary-actions">
                      <button
                        type="button"
                        className="btn-revert-file btn-revert-file-disabled"
                        disabled={true}
                        title="Compiled machine binaries and gitignored executables cannot be reverse-patched"
                      >
                        <RotateCcwIcon size={12} />
                        <span>Rollback Unavailable for Binary Payload</span>
                      </button>
                    </div>
                  )}
                </div>
              </div>
            </div>
          ) : !currentDiff ? (
            <div className="diff-empty-state">
              <span>No modifications detected for <code>{activeFile || 'selected file'}</code>.</span>
            </div>
          ) : (
            <div className="diff-viewer">
              {/* Diff Viewer Action Bar */}
              <div className="diff-viewer-bar">
                <div className="viewer-bar-left">
                  <span className="diff-filename mono">{activeFile}</span>
                  <span className="diff-line-count mono">
                    {viewMode === 'unified' ? `${parsedLines.length} lines` : `${splitRows.length} rows (split)`}
                  </span>
                  {revertMessage && (
                    <span className="diff-revert-toast mono">{revertMessage}</span>
                  )}
                  {revertError && (
                    <span className="diff-revert-toast diff-revert-toast-error mono">{revertError}</span>
                  )}
                </div>

                <div className="viewer-bar-right">
                  {/* Surgical Single-File Rollback Button */}
                  {runId && (
                    <button
                      type="button"
                      className={`btn-revert-file ${revertConfirm ? 'confirming' : ''} ${isFileReverted ? 'reverted' : ''}`}
                      onClick={handleRevertFile}
                      disabled={isReverting || isFileReverted}
                      title={isFileReverted ? 'File has already been reverted' : 'Revert only this file to previous state'}
                    >
                      <RotateCcwIcon size={12} className={isReverting ? 'spin-ccw' : ''} />
                      <span>
                        {isFileReverted
                          ? 'Reverted ✓'
                          : revertConfirm
                          ? 'Confirm Revert?'
                          : 'Revert File'}
                      </span>
                    </button>
                  )}

                  {/* Split vs Unified Mode Toggle */}
                  <div className="diff-view-segmented" role="group" aria-label="Diff View Mode">
                    <button
                      type="button"
                      className={`btn-segment ${viewMode === 'unified' ? 'active' : ''}`}
                      onClick={() => handleSetViewMode('unified')}
                      title="Unified (Inline) Diff"
                    >
                      <RowsIcon size={12} />
                      <span>Unified</span>
                    </button>
                    <button
                      type="button"
                      className={`btn-segment ${viewMode === 'split' ? 'active' : ''}`}
                      onClick={() => handleSetViewMode('split')}
                      title="Split (Side-by-Side) Diff"
                    >
                      <ColumnsIcon size={12} />
                      <span>Split</span>
                    </button>
                  </div>
                </div>
              </div>

              {/* Diff Table View */}
              <div className="diff-table-scroller">
                {viewMode === 'unified' ? (
                  /* Unified Table */
                  <table className="diff-table">
                    <tbody>
                      {parsedLines.map((line, idx) => {
                        if (line.type === 'hunk') {
                          return (
                            <tr key={idx} className="diff-row diff-row-hunk">
                              <td colSpan={3} className="diff-hunk-cell mono">
                                <div className="diff-hunk-inner">
                                  <span className="hunk-badge">Hunk</span>
                                  <span className="hunk-content">{line.content}</span>
                                </div>
                              </td>
                            </tr>
                          )
                        }

                        if (line.type === 'meta') {
                          return (
                            <tr key={idx} className="diff-row diff-row-meta">
                              <td colSpan={3} className="diff-meta-cell mono">
                                {line.content}
                              </td>
                            </tr>
                          )
                        }

                        const isAdd = line.type === 'add'
                        const isDel = line.type === 'del'
                        const prefix = isAdd ? '+' : isDel ? '-' : ' '

                        return (
                          <tr
                            key={idx}
                            className={`diff-row ${isAdd ? 'diff-row-add' : isDel ? 'diff-row-del' : 'diff-row-ctx'}`}
                          >
                            <td className="diff-num mono">{line.oldNum !== undefined ? line.oldNum : ''}</td>
                            <td className="diff-num mono">{line.newNum !== undefined ? line.newNum : ''}</td>
                            <td className="diff-code mono">
                              <span className="diff-prefix">{prefix}</span>
                              <span className="diff-text">{line.content}</span>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                ) : (
                  /* Side-by-Side Split Table */
                  <table className="diff-table diff-table-split">
                    <thead>
                      <tr className="diff-split-header-row">
                        <th colSpan={2} className="diff-split-th diff-split-th-left">
                          <span>Original (Old)</span>
                        </th>
                        <th colSpan={2} className="diff-split-th diff-split-th-right">
                          <span>Modified (New)</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {splitRows.map((row, idx) => {
                        if (row.type === 'hunk') {
                          return (
                            <tr key={idx} className="diff-row diff-row-hunk">
                              <td colSpan={4} className="diff-hunk-cell mono">
                                <div className="diff-hunk-inner">
                                  <span className="hunk-badge">Hunk</span>
                                  <span className="hunk-content">{row.hunkContent}</span>
                                </div>
                              </td>
                            </tr>
                          )
                        }

                        if (row.type === 'meta') {
                          return (
                            <tr key={idx} className="diff-row diff-row-meta">
                              <td colSpan={4} className="diff-meta-cell mono">
                                {row.metaContent}
                              </td>
                            </tr>
                          )
                        }

                        const left = row.left
                        const right = row.right

                        return (
                          <tr key={idx} className="diff-row diff-row-split">
                            {/* Left Line Number */}
                            <td className={`diff-num diff-split-num mono ${left?.type === 'del' ? 'num-del' : ''}`}>
                              {left?.num !== undefined ? left.num : ''}
                            </td>

                            {/* Left Content */}
                            <td
                              className={`diff-code diff-split-code diff-split-code-left mono ${
                                left ? (left.type === 'del' ? 'diff-cell-del' : 'diff-cell-ctx') : 'diff-cell-empty'
                              }`}
                            >
                              {left ? (
                                <>
                                  <span className="diff-prefix">{left.type === 'del' ? '-' : ' '}</span>
                                  <span className="diff-text">{left.content}</span>
                                </>
                              ) : null}
                            </td>

                            {/* Right Line Number */}
                            <td className={`diff-num diff-split-num mono ${right?.type === 'add' ? 'num-add' : ''}`}>
                              {right?.num !== undefined ? right.num : ''}
                            </td>

                            {/* Right Content */}
                            <td
                              className={`diff-code diff-split-code diff-split-code-right mono ${
                                right ? (right.type === 'add' ? 'diff-cell-add' : 'diff-cell-ctx') : 'diff-cell-empty'
                              }`}
                            >
                              {right ? (
                                <>
                                  <span className="diff-prefix">{right.type === 'add' ? '+' : ' '}</span>
                                  <span className="diff-text">{right.content}</span>
                                </>
                              ) : null}
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
