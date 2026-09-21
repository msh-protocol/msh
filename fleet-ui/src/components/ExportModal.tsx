import { useState, useEffect, useMemo } from 'react'
import { DownloadIcon, CopyIcon, CheckIcon, XIcon, FileTextIcon, FolderIcon } from './Icons'
import './ExportModal.css'

interface ExportModalProps {
  isOpen: boolean
  onClose: () => void
  records: any[]
  totalCount: number
  token: string
}

export function ExportModal({
  isOpen,
  onClose,
  records = [],
  totalCount,
  token
}: ExportModalProps) {
  const [format, setFormat] = useState<'json' | 'csv' | 'log'>('json')
  const [scope, setScope] = useState<'filtered' | 'all'>('filtered')
  const [copied, setCopied] = useState<boolean>(false)
  const [isExporting, setIsExporting] = useState<boolean>(false)
  const [statusMsg, setStatusMsg] = useState<string>('')

  const today = useMemo(() => new Date().toISOString().slice(0, 10), [])
  const [customName, setCustomName] = useState<string>(`msh-history-${today}`)

  // Update extension when format changes
  const fullFileName = `${customName}.${format === 'log' ? 'txt' : format}`

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const getExportData = async (): Promise<any[]> => {
    if (scope === 'filtered') {
      return records
    }
    const res = await fetch(`/api/history?limit=1000&offset=0`, {
      headers: token ? { 'Authorization': `Bearer ${token}` } : {}
    })
    if (res.ok) {
      const rawData = await res.json()
      return rawData.map((r: any) => {
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
    }
    return records
  }

  const generateContent = (data: any[]): { content: string; mimeType: string } => {
    if (format === 'json') {
      const jsonPayload = data.map(r => ({
        id: r.ID,
        timestamp: r.Timestamp,
        session_id: r.SessionID,
        command: r.Command,
        status: r.Status,
        exit_code: r.ExitCode,
        duration_ms: r.DurationMs,
        cwd: r.cwd,
        files_changed: r.filesChanged,
        file_diffs: r.fileDiffs,
        error_root_cause: r.rootCause,
        run_hash: r.runHash,
        request: r.parsedReq,
        response: r.parsedResp,
      }))
      return {
        content: JSON.stringify(jsonPayload, null, 2),
        mimeType: 'application/json'
      }
    }

    if (format === 'csv') {
      const headers = [
        'ID',
        'Timestamp',
        'Status',
        'ExitCode',
        'DurationMs',
        'Command',
        'Cwd',
        'FilesChangedCount',
        'HasDiffs',
        'RootCauseType',
        'SessionID',
        'RunHash'
      ]
      const rows = data.map(r => [
        r.ID,
        r.Timestamp,
        r.Status,
        r.ExitCode,
        r.DurationMs,
        r.Command,
        r.cwd,
        r.filesChanged?.length || 0,
        Object.keys(r.fileDiffs || {}).length > 0 ? 'true' : 'false',
        r.rootCause?.type || '',
        r.SessionID,
        r.runHash
      ].map(val => `"${String(val ?? '').replace(/"/g, '""')}"`).join(','))
      return {
        content: [headers.join(','), ...rows].join('\r\n'),
        mimeType: 'text/csv'
      }
    }

    // Text Log format
    const lines = data.map(r => {
      const time = r.Timestamp || ''
      const status = r.Status?.toUpperCase() || 'UNKNOWN'
      const dur = r.DurationMs ? `${r.DurationMs}ms` : ''
      const out = r.parsedResp?.stdout ? `\n--- Output ---\n${r.parsedResp.stdout.trim()}` : ''
      const err = r.parsedResp?.stderr ? `\n--- Stderr ---\n${r.parsedResp.stderr.trim()}` : ''
      return `[${time}] [#${r.ID}] ${status} (exit ${r.ExitCode}, ${dur})\n$ ${r.Command}${out}${err}\n`
    })
    return {
      content: lines.join('\n' + '='.repeat(60) + '\n\n'),
      mimeType: 'text/plain'
    }
  }

  // Choose Destination Folder & Save (opens native Windows file dialog)
  const handleChooseFolderAndSave = async () => {
    setIsExporting(true)
    setStatusMsg('Opening folder chooser...')
    try {
      const data = await getExportData()
      const { content, mimeType } = generateContent(data)
      const ext = format === 'log' ? '.txt' : `.${format}`
      const desc = format === 'json' ? 'JSON Data File' : format === 'csv' ? 'CSV Spreadsheet' : 'Text Log File'

      // Modern File System Access API (allows choosing any folder on disk)
      if ('showSaveFilePicker' in window) {
        try {
          const handle = await (window as any).showSaveFilePicker({
            suggestedName: fullFileName,
            types: [{
              description: desc,
              accept: { [mimeType]: [ext] }
            }]
          })
          const writable = await handle.createWritable()
          await writable.write(content)
          await writable.close()
          setStatusMsg(`✓ Successfully saved to ${handle.name}`)
          setTimeout(() => {
            setStatusMsg('')
            onClose()
          }, 1800)
          return
        } catch (pickerErr: any) {
          if (pickerErr.name === 'AbortError') {
            setIsExporting(false)
            setStatusMsg('')
            return
          }
          console.warn('File picker error, falling back to standard download:', pickerErr)
        }
      }

      // Standard Blob Download Fallback
      triggerBlobDownload(content, mimeType, fullFileName)
    } catch (err: any) {
      setStatusMsg(`Export failed: ${err.message || err}`)
    } finally {
      setIsExporting(false)
    }
  }

  // Quick download directly to browser downloads directory
  const handleQuickDownload = async () => {
    setIsExporting(true)
    setStatusMsg('Exporting...')
    try {
      const data = await getExportData()
      const { content, mimeType } = generateContent(data)
      triggerBlobDownload(content, mimeType, fullFileName)
    } catch (err: any) {
      setStatusMsg(`Download failed: ${err.message || err}`)
    } finally {
      setIsExporting(false)
    }
  }

  const triggerBlobDownload = (content: string, mimeType: string, filename: string) => {
    const blob = new Blob([content], { type: `${mimeType};charset=utf-8` })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)

    setStatusMsg(`✓ Downloaded ${filename}`)
    setTimeout(() => {
      setStatusMsg('')
      onClose()
    }, 1800)
  }

  const handleCopy = async () => {
    setIsExporting(true)
    try {
      const data = await getExportData()
      const { content } = generateContent(data)
      await navigator.clipboard.writeText(content)
      setCopied(true)
      setTimeout(() => setCopied(false), 2200)
    } catch (err: any) {
      setStatusMsg(`Copy failed: ${err.message || err}`)
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <div className="export-modal-backdrop" onClick={onClose}>
      <div className="export-modal-card" onClick={e => e.stopPropagation()}>
        {/* Header */}
        <div className="export-modal-header">
          <div className="export-title-wrap">
            <span className="export-icon-badge">
              <DownloadIcon size={16} />
            </span>
            <div>
              <h3 className="export-modal-title">Export & Download Chooser</h3>
              <p className="export-modal-sub">
                Select format and download destination for your execution telemetry archive.
              </p>
            </div>
          </div>
          <button type="button" className="btn-modal-close" onClick={onClose} title="Close (Esc)">
            <XIcon size={14} />
          </button>
        </div>

        {/* Modal Body */}
        <div className="export-modal-body">
          {/* Format Selection Cards */}
          <div className="export-section">
            <label className="export-section-label mono">EXPORT FORMAT</label>
            <div className="export-format-grid">
              <button
                type="button"
                className={`format-card ${format === 'json' ? 'active' : ''}`}
                onClick={() => setFormat('json')}
              >
                <div className="format-badge mono">JSON</div>
                <div className="format-name">Full Structured Payloads</div>
                <div className="format-desc">Complete request & response objects, diffs, and timing.</div>
              </button>

              <button
                type="button"
                className={`format-card ${format === 'csv' ? 'active' : ''}`}
                onClick={() => setFormat('csv')}
              >
                <div className="format-badge mono">CSV</div>
                <div className="format-name">Spreadsheet Table</div>
                <div className="format-desc">Row-by-row metrics for Excel, Google Sheets, or Pandas.</div>
              </button>

              <button
                type="button"
                className={`format-card ${format === 'log' ? 'active' : ''}`}
                onClick={() => setFormat('log')}
              >
                <div className="format-badge mono">LOG</div>
                <div className="format-name">Human-Readable Logs</div>
                <div className="format-desc">Timestamped console output stream with separators.</div>
              </button>
            </div>
          </div>

          {/* Scope Selector */}
          <div className="export-section">
            <label className="export-section-label mono">RECORDS SCOPE</label>
            <div className="export-scope-toggle">
              <button
                type="button"
                className={`btn-scope ${scope === 'filtered' ? 'active' : ''}`}
                onClick={() => setScope('filtered')}
              >
                <span>Current Filtered View ({records.length} runs)</span>
              </button>
              <button
                type="button"
                className={`btn-scope ${scope === 'all' ? 'active' : ''}`}
                onClick={() => setScope('all')}
              >
                <span>All Cluster History ({totalCount} total)</span>
              </button>
            </div>
          </div>

          {/* File Name & Path Preview */}
          <div className="export-section">
            <label className="export-section-label mono">DESTINATION FILENAME</label>
            <div className="export-filename-box">
              <FileTextIcon size={14} className="file-icon" />
              <input
                type="text"
                className="export-filename-input mono"
                value={customName}
                onChange={e => setCustomName(e.target.value)}
                placeholder="filename"
              />
              <span className="export-ext-badge mono">.{format === 'log' ? 'txt' : format}</span>
            </div>
            <div className="export-hint mono">
              <FolderIcon size={12} />
              <span>Click "Choose Folder & Save" to pick any destination folder on your computer.</span>
            </div>
          </div>

          {statusMsg && (
            <div className="export-status-toast mono">
              {statusMsg}
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="export-modal-footer">
          <div className="footer-left">
            <button
              type="button"
              className="btn-modal-copy"
              onClick={handleCopy}
              disabled={isExporting}
              title="Copy formatted export text directly to clipboard"
            >
              {copied ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
              <span>{copied ? 'Copied to Clipboard' : 'Copy Payload'}</span>
            </button>
          </div>

          <div className="footer-right">
            <button type="button" className="btn-modal-cancel" onClick={onClose}>
              Cancel
            </button>
            <button
              type="button"
              className="btn-modal-quick"
              onClick={handleQuickDownload}
              disabled={isExporting}
              title="Save directly into default Downloads directory"
            >
              <DownloadIcon size={12} />
              <span>Quick Download</span>
            </button>
            <button
              type="button"
              className="btn-modal-download"
              onClick={handleChooseFolderAndSave}
              disabled={isExporting}
              title="Open folder chooser dialog to select save location"
            >
              <FolderIcon size={13} />
              <span>{isExporting ? 'Exporting...' : 'Choose Folder & Save'}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
