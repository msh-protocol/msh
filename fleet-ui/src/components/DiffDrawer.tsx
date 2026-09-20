import { useState, useEffect, useMemo, useCallback } from 'react'

interface DiffDrawerProps {
  isOpen: boolean
  onClose: () => void
  files: string[]
  diffs?: Record<string, string>
  initialFile?: string
  token?: string
  cwd?: string
}

interface ParsedDiffLine {
  type: 'hunk' | 'add' | 'del' | 'context' | 'meta'
  oldNum?: number
  newNum?: number
  content: string
}

export function DiffDrawer({
  isOpen,
  onClose,
  files = [],
  diffs = {},
  initialFile,
  token,
  cwd
}: DiffDrawerProps) {
  const [activeFile, setActiveFile] = useState<string>(() => initialFile || files[0] || '')
  const [fileDiffMap, setFileDiffMap] = useState<Record<string, string>>(diffs)
  const [loading, setLoading] = useState<boolean>(false)
  const [copied, setCopied] = useState<boolean>(false)

  // Sync incoming diffs or initialFile
  useEffect(() => {
    setFileDiffMap(prev => ({ ...diffs, ...prev }))
  }, [diffs])

  useEffect(() => {
    if (initialFile && files.includes(initialFile)) {
      setActiveFile(initialFile)
    } else if (!activeFile && files.length > 0) {
      setActiveFile(files[0])
    }
  }, [initialFile, files, activeFile])

  // Fetch diff on-demand if not already present
  const fetchMissingDiff = useCallback(async (file: string) => {
    if (!file || fileDiffMap[file]) return
    setLoading(true)
    try {
      const q = new URLSearchParams({ file })
      if (cwd) q.set('cwd', cwd)
      const res = await fetch(`/api/diff?${q.toString()}`, {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {}
      })
      if (res.ok) {
        const data = await res.json()
        setFileDiffMap(prev => ({ ...prev, [file]: data.diff || '' }))
      }
    } catch (err) {
      console.error('Failed to fetch diff for file', file, err)
    } finally {
      setLoading(false)
    }
  }, [cwd, fileDiffMap, token])

  useEffect(() => {
    if (isOpen && activeFile && !fileDiffMap[activeFile]) {
      fetchMissingDiff(activeFile)
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

  const currentDiff = fileDiffMap[activeFile] || ''

  // Parse diff lines with dual line numbering
  const parsedLines = useMemo(() => {
    if (!currentDiff) return []
    const rawLines = currentDiff.split('\n')
    const result: ParsedDiffLine[] = []
    let oldNum = 0
    let newNum = 0

    for (const line of rawLines) {
      if (line.startsWith('@@')) {
        // Hunk header: @@ -oldStart,oldCount +newStart,newCount @@
        const match = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/)
        if (match) {
          oldNum = parseInt(match[1], 10)
          newNum = parseInt(match[2], 10)
        }
        result.push({ type: 'hunk', content: line })
      } else if (line.startsWith('---') || line.startsWith('+++') || line.startsWith('diff --git')) {
        result.push({ type: 'meta', content: line })
      } else if (line.startsWith('+')) {
        result.push({ type: 'add', newNum: newNum++, content: line.slice(1) })
      } else if (line.startsWith('-')) {
        result.push({ type: 'del', oldNum: oldNum++, content: line.slice(1) })
      } else {
        // Context line
        const content = line.startsWith(' ') ? line.slice(1) : line
        result.push({ type: 'context', oldNum: oldNum++, newNum: newNum++, content })
      }
    }
    return result
  }, [currentDiff])

  // Calculate file-level and total stats
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

  if (!isOpen) return null

  return (
    <div className="diff-drawer-backdrop" onClick={onClose}>
      <div className="diff-drawer" onClick={e => e.stopPropagation()}>
        {/* Header */}
        <div className="diff-drawer-header">
          <div className="diff-header-left">
            <span className="diff-icon">📄</span>
            <div>
              <h3 className="diff-title">Files Changed</h3>
              <span className="diff-subtitle">
                {files.length} {files.length === 1 ? 'file' : 'files'} modified
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
            <button
              className="btn btn-secondary btn-sm"
              onClick={copyDiff}
              disabled={!currentDiff}
              title="Copy unified diff to clipboard"
            >
              {copied ? '✓ Copied' : 'Copy Diff'}
            </button>
            <button className="diff-close-btn" onClick={onClose} title="Close (Esc)">
              ✕
            </button>
          </div>
        </div>

        {/* File Navigator Tabs */}
        {files.length > 0 && (
          <div className="diff-file-tabs">
            {files.map(f => {
              const diffText = fileDiffMap[f] || ''
              const isDeleted = diffText.includes('+++ /dev/null')
              const isAdded = diffText.includes('--- /dev/null')
              let badgeType = 'M'
              if (isAdded) badgeType = 'A'
              if (isDeleted) badgeType = 'D'

              return (
                <button
                  key={f}
                  className={`diff-tab-btn ${activeFile === f ? 'active' : ''}`}
                  onClick={() => setActiveFile(f)}
                  title={f}
                >
                  <span className={`file-badge file-badge-${badgeType.toLowerCase()}`}>
                    {badgeType}
                  </span>
                  <span className="diff-tab-name">{f}</span>
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
              <span>Computing git diff...</span>
            </div>
          ) : !currentDiff ? (
            <div className="diff-empty-state">
              <span>No diff available for <code>{activeFile || 'selected file'}</code>.</span>
            </div>
          ) : (
            <div className="diff-viewer">
              <div className="diff-viewer-bar">
                <span className="diff-filename mono">{activeFile}</span>
                <span className="diff-line-count mono">{parsedLines.length} lines</span>
              </div>
              <table className="diff-table">
                <tbody>
                  {parsedLines.map((line, idx) => {
                    if (line.type === 'hunk') {
                      return (
                        <tr key={idx} className="diff-row diff-row-hunk">
                          <td colSpan={3} className="diff-hunk-cell mono">
                            {line.content}
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
                        <td className="diff-num mono">{line.oldNum ?? ''}</td>
                        <td className="diff-num mono">{line.newNum ?? ''}</td>
                        <td className="diff-code mono">
                          <span className="diff-prefix">{prefix}</span>
                          <span className="diff-text">{line.content}</span>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
