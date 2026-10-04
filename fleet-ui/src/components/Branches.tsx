import { useState, useEffect } from 'react'
import {
  GitBranchIcon,
  PlusIcon,
  PlayIcon,
  FileTextIcon,
  CheckIcon,
  TrashIcon,
  RotateCcwIcon,
  AlertTriangleIcon,
  XIcon
} from './Icons'
import './Branches.css'

export interface BranchInfo {
  name: string
  git_branch: string
  path: string
  base_ref: string
  base_commit: string
  created_at: string
  status: 'active' | 'merged' | 'aborted'
  description?: string
  runs_count: number
  modified_files: string[]
}

export function Branches({ token }: { token: string }) {
  const [branches, setBranches] = useState<BranchInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Creation State
  const [isCreating, setIsCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [newFrom, setNewFrom] = useState('HEAD')
  const [newDesc, setNewDesc] = useState('')
  const [creatingSubmitting, setCreatingSubmitting] = useState(false)

  // In-Worktree Command Execution State
  const [cmdInputs, setCmdInputs] = useState<Record<string, string>>({})
  const [executingBranch, setExecutingBranch] = useState<string | null>(null)
  const [execOutputs, setExecOutputs] = useState<Record<string, { stdout: string; stderr: string; exitCode: number }>>({})

  // Diff Modal State
  const [viewingDiffBranch, setViewingDiffBranch] = useState<string | null>(null)
  const [diffContent, setDiffContent] = useState<string>('')
  const [loadingDiff, setLoadingDiff] = useState(false)

  // Prune / Abort Feedback Banner
  const [pruneAlert, setPruneAlert] = useState<{ branch: string; turns: number; filesCount: number } | null>(null)

  // In-App Confirmation Modal State (replaces window.confirm & alert)
  const [confirmModal, setConfirmModal] = useState<{
    type: 'merge' | 'abort'
    branch: string
    loading?: boolean
    error?: string | null
  } | null>(null)

  const fetchBranches = async () => {
    try {
      setLoading(true)
      const res = await fetch('/api/branches', {
        headers: token ? { Authorization: `Bearer ${token}` } : {}
      })
      if (res.ok) {
        const data = await res.json()
        setBranches(Array.isArray(data) ? data : [])
        setError(null)
      } else {
        setError('Failed to fetch speculative shadow branches.')
      }
    } catch (e: any) {
      setError(e.message || 'Connection error.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchBranches()
  }, [token])

  const handleCreateBranch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return

    setCreatingSubmitting(true)
    try {
      const res = await fetch('/api/branch/create', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {})
        },
        body: JSON.stringify({
          name: newName.trim(),
          from: newFrom.trim() || 'HEAD',
          desc: newDesc.trim()
        })
      })

      if (res.ok) {
        setNewName('')
        setNewDesc('')
        setIsCreating(false)
        await fetchBranches()
      } else {
        const errText = await res.text()
        setError(`Failed to create shadow branch: ${errText}`)
      }
    } catch (err: any) {
      setError(`Error: ${err.message}`)
    } finally {
      setCreatingSubmitting(false)
    }
  }

  const handleRunCommand = async (branchName: string) => {
    const cmd = cmdInputs[branchName]
    if (!cmd || !cmd.trim()) {
      setExecOutputs(prev => ({
        ...prev,
        [branchName]: {
          stdout: '',
          stderr: 'Please type a command into the input field above (e.g. npm test, go test ./...)',
          exitCode: 1
        }
      }))
      return
    }

    setExecutingBranch(branchName)
    try {
      const res = await fetch('/api/branch/run', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {})
        },
        body: JSON.stringify({
          name: branchName,
          command: cmd.trim()
        })
      })

      if (res.ok) {
        const data = await res.json()
        setExecOutputs(prev => ({
          ...prev,
          [branchName]: {
            stdout: data.stdout || '',
            stderr: data.stderr || '',
            exitCode: data.exit_code ?? 0
          }
        }))
        // Refresh branch to show updated modified files and run counts
        await fetchBranches()
      } else {
        const err = await res.text()
        setExecOutputs(prev => ({
          ...prev,
          [branchName]: {
            stdout: '',
            stderr: err,
            exitCode: 1
          }
        }))
      }
    } catch (err: any) {
      setExecOutputs(prev => ({
        ...prev,
        [branchName]: {
          stdout: '',
          stderr: err.message,
          exitCode: 1
        }
      }))
    } finally {
      setExecutingBranch(null)
    }
  }

  const handleViewDiff = async (branchName: string) => {
    setViewingDiffBranch(branchName)
    setLoadingDiff(true)
    setDiffContent('')
    try {
      const res = await fetch(`/api/branch/diff?name=${encodeURIComponent(branchName)}`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {}
      })
      if (res.ok) {
        const data = await res.json()
        setDiffContent(data.diff || 'No modifications detected in this shadow worktree.')
      } else {
        setDiffContent('Failed to generate diff.')
      }
    } catch {
      setDiffContent('Connection error while retrieving diff.')
    } finally {
      setLoadingDiff(false)
    }
  }

  const handleMergeBranch = (branchName: string) => {
    setConfirmModal({ type: 'merge', branch: branchName, loading: false, error: null })
  }

  const handleAbortBranch = (branchName: string) => {
    setConfirmModal({ type: 'abort', branch: branchName, loading: false, error: null })
  }

  const executeConfirmAction = async () => {
    if (!confirmModal) return
    const { type, branch } = confirmModal
    setConfirmModal(prev => prev ? { ...prev, loading: true, error: null } : null)

    try {
      if (type === 'merge') {
        const res = await fetch('/api/branch/merge', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {})
          },
          body: JSON.stringify({
            name: branch,
            message: `msh: verified solution from shadow branch '${branch}'`
          })
        })

        if (res.ok) {
          setConfirmModal(null)
          await fetchBranches()
        } else {
          const err = await res.text()
          setConfirmModal(prev => prev ? { ...prev, loading: false, error: `Failed to merge: ${err}` } : null)
        }
      } else {
        const res = await fetch('/api/branch/abort', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {})
          },
          body: JSON.stringify({ name: branch })
        })

        if (res.ok) {
          const pruneData = await res.json()
          setPruneAlert({
            branch: branch,
            turns: pruneData.context_prune_turns || 1,
            filesCount: pruneData.files_cleaned?.length || 0
          })
          setConfirmModal(null)
          await fetchBranches()
        } else {
          const err = await res.text()
          setConfirmModal(prev => prev ? { ...prev, loading: false, error: `Failed to abort: ${err}` } : null)
        }
      }
    } catch (e: any) {
      setConfirmModal(prev => prev ? { ...prev, loading: false, error: `Network error: ${e.message}` } : null)
    }
  }

  return (
    <div className="branches-container">
      {/* Header Banner */}
      <div className="branches-header">
        <div className="branches-header-left">
          <div className="branch-icon-badge">
            <GitBranchIcon size={24} />
          </div>
          <div>
            <div className="branch-title-row">
              <h2 className="branches-title">Speculative Execution & Shadow Worktrees</h2>
              <span className="branches-version-pill">v1.5.0 Runtime Isolation</span>
            </div>
            <p className="branches-subtitle">
              Sub-second transactional git worktrees for parallel hypothesis exploration (arXiv:2512.12806). 
              Allow autonomous agents to test multiple refactors simultaneously without polluting your working tree.
            </p>
          </div>
        </div>

        <div className="branches-header-actions">
          <button
            type="button"
            className="btn-create-branch"
            onClick={() => setIsCreating(true)}
          >
            <PlusIcon size={14} />
            <span>New Shadow Branch</span>
          </button>
        </div>
      </div>

      {/* Semantic Pruning Alert Banner */}
      {pruneAlert && (
        <div className="prune-alert-banner">
          <div className="prune-alert-content">
            <span className="prune-alert-icon">
              <RotateCcwIcon size={15} />
            </span>
            <div>
              <strong>Hypothesis '{pruneAlert.branch}' discarded:</strong> Main workspace preserved 100% clean. 
              Suggested agent context rewind: <span className="mono bold">{pruneAlert.turns} turns</span> (arXiv:2608.03836).
            </div>
          </div>
          <button
            type="button"
            className="btn-close-banner"
            onClick={() => setPruneAlert(null)}
          >
            <XIcon size={14} />
          </button>
        </div>
      )}

      {/* Modal: Create Shadow Branch */}
      {isCreating && (
        <div className="branch-modal-overlay">
          <div className="branch-modal-card">
            <div className="modal-header">
              <div className="modal-title-group">
                <GitBranchIcon size={18} className="modal-title-icon" />
                <h3>Spin Up Shadow Worktree</h3>
              </div>
              <button
                type="button"
                className="btn-close-modal"
                onClick={() => setIsCreating(false)}
              >
                <XIcon size={15} />
              </button>
            </div>

            <form onSubmit={handleCreateBranch} className="branch-form">
              <div className="form-field">
                <label>Branch Identifier</label>
                <input
                  type="text"
                  placeholder="e.g. opt-query-plan, try-ast-transform"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  className="mono"
                  autoFocus
                  required
                />
                <span className="field-hint">Alphanumeric, dashes, and underscores only.</span>
              </div>

              <div className="form-field">
                <label>Base Reference</label>
                <input
                  type="text"
                  placeholder="HEAD"
                  value={newFrom}
                  onChange={(e) => setNewFrom(e.target.value)}
                  className="mono"
                />
                <span className="field-hint">Commit hash, tag, or branch to branch from.</span>
              </div>

              <div className="form-field">
                <label>Hypothesis Purpose</label>
                <input
                  type="text"
                  placeholder="e.g. Test alternative async streaming parser"
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                />
              </div>

              <div className="modal-actions">
                <button
                  type="button"
                  className="btn-cancel"
                  onClick={() => setIsCreating(false)}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="btn-submit"
                  disabled={creatingSubmitting || !newName.trim()}
                >
                  {creatingSubmitting ? 'Provisioning...' : 'Provision Shadow Worktree'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Diff Inspector Modal */}
      {viewingDiffBranch && (
        <div className="branch-modal-overlay">
          <div className="diff-modal-card">
            <div className="modal-header">
              <div className="modal-title-group">
                <FileTextIcon size={18} className="modal-title-icon" />
                <h3>Diff: Shadow Branch '{viewingDiffBranch}'</h3>
              </div>
              <button
                type="button"
                className="btn-close-modal"
                onClick={() => setViewingDiffBranch(null)}
              >
                <XIcon size={15} />
              </button>
            </div>

            <div className="diff-modal-body">
              {loadingDiff ? (
                <div className="diff-loading mono">Calculating unified git diff...</div>
              ) : (
                <pre className="diff-pre mono">{diffContent}</pre>
              )}
            </div>
          </div>
        </div>
      )}

      {/* In-App Confirmation Modal (Merge / Abort) */}
      {confirmModal && (
        <div className="branch-modal-overlay">
          <div className="branch-modal-card" style={{ maxWidth: '440px' }}>
            <div className="modal-header">
              <div className="modal-title-group">
                {confirmModal.type === 'merge' ? (
                  <CheckIcon size={18} className="modal-title-icon" style={{ color: 'var(--success, #385a49)' }} />
                ) : (
                  <TrashIcon size={18} className="modal-title-icon" style={{ color: 'var(--danger, #8a3328)' }} />
                )}
                <h3>
                  {confirmModal.type === 'merge' ? 'Merge Speculative Solution' : 'Discard Speculative Hypothesis'}
                </h3>
              </div>
              <button
                type="button"
                className="btn-close-modal"
                disabled={confirmModal.loading}
                onClick={() => !confirmModal.loading && setConfirmModal(null)}
              >
                <XIcon size={15} />
              </button>
            </div>

            <div style={{ padding: '20px' }}>
              <p style={{ margin: '0 0 16px', fontSize: '13px', lineHeight: '1.5', color: 'var(--text-secondary, #574f47)' }}>
                {confirmModal.type === 'merge' ? (
                  <>
                    Merge verified changes from shadow branch <strong className="mono">{confirmModal.branch}</strong> into your workspace? This will apply all diffs to your primary working tree and prune the shadow worktree.
                  </>
                ) : (
                  <>
                    Discard speculative hypothesis <strong className="mono">{confirmModal.branch}</strong>? This will cleanly remove the shadow worktree and discard all mutations without touching your main workspace.
                  </>
                )}
              </p>

              {confirmModal.error && (
                <div className="branches-error-state" style={{ marginBottom: '16px', padding: '10px 14px' }}>
                  <AlertTriangleIcon size={15} />
                  <span style={{ fontSize: '12px' }}>{confirmModal.error}</span>
                </div>
              )}

              <div className="modal-actions">
                <button
                  type="button"
                  className="btn-cancel"
                  disabled={confirmModal.loading}
                  onClick={() => setConfirmModal(null)}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="btn-submit"
                  style={confirmModal.type === 'abort' ? {
                    background: 'var(--danger, #8a3328)',
                    borderColor: 'var(--danger, #8a3328)'
                  } : {
                    background: 'var(--success, #385a49)',
                    borderColor: 'var(--success, #385a49)'
                  }}
                  disabled={confirmModal.loading}
                  onClick={executeConfirmAction}
                >
                  {confirmModal.loading
                    ? (confirmModal.type === 'merge' ? 'Merging Solution...' : 'Aborting...')
                    : (confirmModal.type === 'merge' ? '✓ Confirm & Merge' : 'Discard & Abort')}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Branches List */}
      {loading ? (
        <div className="branches-empty-state">
          <p className="mono">Loading speculative shadow branches...</p>
        </div>
      ) : error ? (
        <div className="branches-error-state">
          <AlertTriangleIcon size={16} />
          <span>{error}</span>
        </div>
      ) : branches.length === 0 ? (
        <div className="branches-empty-state">
          <GitBranchIcon size={32} className="empty-icon" />
          <h3>No Active Shadow Branches</h3>
          <p>
            Spin up a sub-second shadow git worktree to explore speculative code refactors, parallel prompt hypotheses, or dangerous tests in zero-risk isolation.
          </p>
          <button
            type="button"
            className="btn-create-branch"
            onClick={() => setIsCreating(true)}
          >
            <PlusIcon size={14} />
            <span>Create First Shadow Branch</span>
          </button>
        </div>
      ) : (
        <div className="branches-grid">
          {branches.map((b) => {
            const output = execOutputs[b.name]
            const isRunning = executingBranch === b.name

            return (
              <div key={b.name} className="branch-card">
                <div className="branch-card-header">
                  <div className="branch-card-title-wrap">
                    <div className="branch-name-row">
                      <span className="branch-name mono">{b.name}</span>
                      <span className="branch-status-pill active">ACTIVE HYPOTHESIS</span>
                    </div>
                    <div className="branch-meta-sub mono">
                      git ref: {b.git_branch} · base: {b.base_commit?.slice(0, 8) || b.base_ref} · {b.runs_count} run(s)
                    </div>
                  </div>

                  <div className="branch-card-controls">
                    <button
                      type="button"
                      className="btn-branch-action btn-diff"
                      onClick={() => handleViewDiff(b.name)}
                      title="View unified diff against base commit"
                    >
                      <FileTextIcon size={13} />
                      <span>Inspect Diff</span>
                    </button>
                    <button
                      type="button"
                      className="btn-branch-action btn-merge"
                      onClick={() => handleMergeBranch(b.name)}
                      title="Merge verified changes into main workspace"
                    >
                      <CheckIcon size={13} />
                      <span>Merge Solution</span>
                    </button>
                    <button
                      type="button"
                      className="btn-branch-action btn-abort"
                      onClick={() => handleAbortBranch(b.name)}
                      title="Discard hypothesis cleanly"
                    >
                      <TrashIcon size={13} />
                      <span>Abort</span>
                    </button>
                  </div>
                </div>

                {b.description && (
                  <p className="branch-desc">{b.description}</p>
                )}

                {/* Modified Files Section */}
                <div className="branch-files-summary">
                  <div className="files-summary-head">
                    <span className="summary-label">SPECULATIVE MUTATIONS:</span>
                    <span className="summary-count mono">
                      {b.modified_files?.length || 0} file(s) modified
                    </span>
                  </div>
                  {b.modified_files && b.modified_files.length > 0 && (
                    <div className="files-chips-list">
                      {b.modified_files.map((file, idx) => (
                        <span key={idx} className="file-chip mono">
                          {file}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {/* In-Worktree Command Runner */}
                <div className="branch-exec-box">
                  <div className="exec-input-row">
                    <span className="exec-prompt mono">$</span>
                    <input
                      type="text"
                      className="exec-input mono"
                      placeholder={`Run in shadow '${b.name}' (e.g. npm test, go test ./...)...`}
                      value={cmdInputs[b.name] || ''}
                      onChange={(e) =>
                        setCmdInputs(prev => ({ ...prev, [b.name]: e.target.value }))
                      }
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') handleRunCommand(b.name)
                      }}
                    />
                    <button
                      type="button"
                      className="btn-exec-run"
                      onClick={() => handleRunCommand(b.name)}
                      disabled={isRunning}
                      title={!cmdInputs[b.name]?.trim() ? "Type a command to run inside this shadow worktree" : "Run command"}
                    >
                      <PlayIcon size={12} />
                      <span>{isRunning ? 'Running...' : 'Execute'}</span>
                    </button>
                  </div>

                  {output && (
                    <div className={`exec-output-card ${output.exitCode === 0 ? 'success' : 'error'}`}>
                      <div className="output-header mono">
                        <span>Exit Code: {output.exitCode}</span>
                        <button
                          type="button"
                          className="btn-clear-out"
                          onClick={() =>
                            setExecOutputs(prev => {
                              const next = { ...prev }
                              delete next[b.name]
                              return next
                            })
                          }
                        >
                          Clear
                        </button>
                      </div>
                      <pre className="output-stream mono">
                        {output.stdout || output.stderr || '(no output)'}
                      </pre>
                    </div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
