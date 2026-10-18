import { useState, useEffect } from 'react'
import {
  ShieldCheckIcon,
  ShieldAlertIcon,
  ZapIcon,
  AlertTriangleIcon,
  CheckIcon,
  SearchIcon
} from './Icons'
import './Guardrails.css'

interface PolicyRule {
  id: string
  match: string
  action: 'block' | 'warn'
  risk: 'critical' | 'high' | 'moderate' | 'low'
  message: string
  description?: string
}

interface GuardrailConfig {
  version?: string
  strict_workspace?: boolean
  allowed_paths?: string[]
  denied_paths?: string[]
  policies: PolicyRule[]
}

const DEFAULT_TIER1_POLICIES: PolicyRule[] = [
  {
    id: 'catastrophic-root-deletion',
    match: 'rm -rf / or rm -rf ~',
    action: 'block',
    risk: 'critical',
    message: 'Blocked destructive root or home directory deletion (rm -rf / or rm -rf ~).',
    description: 'Catastrophic deletion floor preventing wiping root directory or agent home.'
  },
  {
    id: 'windows-root-wipe',
    match: 'rmdir /s /q C:\\',
    action: 'block',
    risk: 'critical',
    message: 'Blocked destructive drive root deletion (rmdir /s /q C:\\).',
    description: 'Guards Windows operating system drives from recursive mass wipe.'
  },
  {
    id: 'shell-fork-bomb',
    match: ':(){ :|:& };:',
    action: 'block',
    risk: 'critical',
    message: 'Blocked classic shell fork bomb denial-of-service pattern.',
    description: 'Prevents CPU and PID process exhaustion from adversarial agent prompts.'
  },
  {
    id: 'raw-disk-overwrite',
    match: 'dd .*of=/dev/(sd|hd|nvme|vd|xvd)',
    action: 'block',
    risk: 'critical',
    message: 'Blocked raw block device overwrite via dd.',
    description: 'Prevents raw partition zeroing or master boot record overwrites.'
  },
  {
    id: 'raw-filesystem-format',
    match: 'mkfs\\.|Format-Volume|format [a-zA-Z]:',
    action: 'block',
    risk: 'critical',
    message: 'Blocked raw disk formatting or partition modification command.',
    description: 'Guards against raw storage formatting utilities.'
  },
  {
    id: 'recursive-root-chmod',
    match: 'chmod -R 777 /',
    action: 'block',
    risk: 'critical',
    message: 'Blocked recursive root permission wipe (chmod -R 777 /).',
    description: 'Prevents making the entire operating system world-writable.'
  }
]

export function Guardrails({ token }: { token: string }) {
  const [config, setConfig] = useState<GuardrailConfig>({
    version: '1.0',
    strict_workspace: false,
    policies: DEFAULT_TIER1_POLICIES
  })
  const [testCmd, setTestCmd] = useState('')
  const [strictSim, setStrictSim] = useState(false)
  const [evaluating, setEvaluating] = useState(false)
  const [evalResult, setEvalResult] = useState<any>(null)
  const [filterRisk, setFilterRisk] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')

  useEffect(() => {
    async function loadPolicies() {
      try {
        const res = await fetch('/api/policies', {
          headers: token ? { 'Authorization': `Bearer ${token}` } : {}
        })
        if (res.ok) {
          const data = await res.json()
          if (data && Array.isArray(data.policies) && data.policies.length > 0) {
            setConfig(data)
          }
        }
      } catch (e) {
        console.warn('Using client-side fallback policies:', e)
      }
    }
    loadPolicies()
  }, [token])

  const handleSimulate = async (cmdToTest?: string) => {
    const cmd = cmdToTest !== undefined ? cmdToTest : testCmd
    if (!cmd.trim()) return

    setEvaluating(true)
    try {
      const res = await fetch('/api/guard/check', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {})
        },
        body: JSON.stringify({
          command: cmd,
          strict_workspace: strictSim
        })
      })
      if (res.ok) {
        const data = await res.json()
        setEvalResult(data)
      } else {
        fallbackEval(cmd)
      }
    } catch {
      fallbackEval(cmd)
    } finally {
      setEvaluating(false)
    }
  }

  const fallbackEval = (cmd: string) => {
    const lower = cmd.toLowerCase()
    const isCatastrophic =
      lower.includes('rm -rf /') ||
      lower.includes('rm -rf ~') ||
      lower.includes('rmdir /s /q c:\\') ||
      lower.includes(':(){ :|:& };:') ||
      lower.includes('dd if=') ||
      lower.includes('chmod -r 777 /')

    if (isCatastrophic) {
      setEvalResult({
        allowed: false,
        risk: 'critical',
        violations: [
          {
            rule_id: 'catastrophic-root-deletion',
            risk: 'critical',
            action: 'block',
            message: 'Blocked destructive command targeting root system resources.'
          }
        ],
        reason: 'Blocked by built-in Tier 1 catastrophic safety floor.'
      })
    } else {
      setEvalResult({
        allowed: true,
        risk: 'low',
        violations: [],
        reason: 'Clean: No security policy violations detected.'
      })
    }
  }

  const sampleAttacks = [
    { label: 'rm -rf /', cmd: 'rm -rf /' },
    { label: 'Drive Wipe (C:\\)', cmd: 'rmdir /s /q C:\\' },
    { label: 'Fork Bomb', cmd: ':(){ :|:& };:' },
    { label: 'dd raw disk wipe', cmd: 'dd if=/dev/zero of=/dev/sda bs=1M' },
    { label: 'chmod -R 777 /', cmd: 'chmod -R 777 /' },
    { label: 'Global npm install', cmd: 'npm install -g malicious-tool' }
  ]

  const filteredPolicies = config.policies.filter(p => {
    if (filterRisk !== 'all' && p.risk !== filterRisk) return false
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase()
      return (
        p.id.toLowerCase().includes(q) ||
        p.message.toLowerCase().includes(q) ||
        p.match.toLowerCase().includes(q) ||
        (p.description || '').toLowerCase().includes(q)
      )
    }
    return true
  })

  return (
    <div className="guardrails-container">
      {/* Header Banner */}
      <div className="guardrails-header">
        <div className="guardrails-header-left">
          <div className="guard-icon-badge">
            <ShieldCheckIcon size={24} />
          </div>
          <div>
            <div className="guard-title-row">
              <h2 className="guardrails-title">Execution Guardrails & Safety Engine</h2>
              <span className="guardrails-version-pill">v1.5.0 Policy Runtime</span>
            </div>
            <p className="guardrails-subtitle">
              Three-tier pre-execution defense preventing destructive AI agent actions, system root wipes, and workspace escapes before process spawning.
            </p>
          </div>
        </div>
        <div className="guardrails-header-actions">
          <a
            href="https://github.com/msh-protocol/msh/blob/main/docs/policies.md"
            target="_blank"
            rel="noopener noreferrer"
            className="guard-doc-btn"
          >
            Read Policy Specs ↗
          </a>
        </div>
      </div>

      {/* Three-Tier Defense Architecture Cards */}
      <div className="defense-tiers-grid">
        <div className="defense-tier-card tier-1">
          <div className="tier-header">
            <span className="tier-badge tier-1-badge">TIER 1 · FLOOR</span>
            <span className="tier-status active">Always Active</span>
          </div>
          <h3 className="tier-title">Catastrophic Safety Floor</h3>
          <p className="tier-desc">
            Hard-coded kernel of 6 non-bypassable safety rules blocking filesystem root deletions, disk wipes, fork bombs, and raw volume formats.
          </p>
          <div className="tier-stat mono">6 Critical Rules Enforced</div>
        </div>

        <div className="defense-tier-card tier-2">
          <div className="tier-header">
            <span className="tier-badge tier-2-badge">TIER 2 · PROJECT</span>
            <span className="tier-status active">Loaded from .msh/policies.yaml</span>
          </div>
          <h3 className="tier-title">Custom Project Guardrails</h3>
          <p className="tier-desc">
            Team-defined pattern matching with customized risk levels (<code className="mono">critical</code>, <code className="mono">high</code>, <code className="mono">moderate</code>) and enforcement actions (<code className="mono">block</code> or <code className="mono">warn</code>).
          </p>
          <div className="tier-stat mono">{config.policies.length} Active Rules Configured</div>
        </div>

        <div className="defense-tier-card tier-3">
          <div className="tier-header">
            <span className="tier-badge tier-3-badge">TIER 3 · CONFINEMENT</span>
            <span className={`tier-status ${config.strict_workspace ? 'active' : 'opt-in'}`}>
              {config.strict_workspace ? 'Active' : 'Opt-in (--strict-workspace)'}
            </span>
          </div>
          <h3 className="tier-title">Strict Workspace Boundary</h3>
          <p className="tier-desc">
            Zero-trust path boundary checking blocking parent directory traversal (<code className="mono">../../</code>) and redirections outside workspace root.
          </p>
          <div className="tier-stat mono">
            Workspace: {config.strict_workspace ? 'Strictly Sandboxed' : 'Permissive / Standard'}
          </div>
        </div>
      </div>

      {/* Interactive Policy Simulator */}
      <div className="guard-simulator-card">
        <div className="simulator-head">
          <div className="simulator-title-group">
            <span className="simulator-icon">
              <ZapIcon size={18} />
            </span>
            <div>
              <h3 className="simulator-title">Pre-Execution Policy Simulator</h3>
              <p className="simulator-sub">
                Test commands against the policy engine in real-time without spawning an OS process.
              </p>
            </div>
          </div>
          <label className="strict-toggle-label">
            <input
              type="checkbox"
              checked={strictSim}
              onChange={(e) => setStrictSim(e.target.checked)}
            />
            <span className="toggle-text mono">Simulate --strict-workspace</span>
          </label>
        </div>

        <div className="simulator-input-row">
          <span className="simulator-prompt mono">$</span>
          <input
            type="text"
            className="simulator-input mono"
            placeholder="Type a shell command (e.g., rm -rf /, npm install -g pwn, git push -f)..."
            value={testCmd}
            onChange={(e) => setTestCmd(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleSimulate()
            }}
          />
          <button
            type="button"
            className="btn-evaluate"
            onClick={() => handleSimulate()}
            disabled={evaluating || !testCmd.trim()}
          >
            {evaluating ? 'Evaluating...' : 'Test Policy'}
          </button>
        </div>

        {/* Quick Sample Attack Buttons */}
        <div className="sample-attacks-row">
          <span className="sample-label">Test samples:</span>
          {sampleAttacks.map((s, idx) => (
            <button
              key={idx}
              type="button"
              className="sample-attack-btn mono"
              onClick={() => {
                setTestCmd(s.cmd)
                handleSimulate(s.cmd)
              }}
            >
              {s.label}
            </button>
          ))}
        </div>

        {/* Evaluation Verdict Result */}
        {evalResult && (
          <div className={`eval-verdict-box ${evalResult.allowed ? 'verdict-allowed' : 'verdict-blocked'}`}>
            <div className="verdict-banner">
              <div className="verdict-main">
                <span className="verdict-badge">
                  {evalResult.allowed ? (
                    evalResult.violations?.length > 0 ? (
                      <>
                        <AlertTriangleIcon size={14} className="verdict-icon" /> POLICY WARNING
                      </>
                    ) : (
                      <>
                        <CheckIcon size={14} className="verdict-icon" /> ALLOWED (CLEAN)
                      </>
                    )
                  ) : (
                    <>
                      <ShieldAlertIcon size={14} className="verdict-icon" /> EXECUTION BLOCKED
                    </>
                  )}
                </span>
                <span className={`risk-pill risk-${(evalResult.risk || 'low').toLowerCase()}`}>
                  {(evalResult.risk || 'low').toUpperCase()} RISK
                </span>
              </div>
              <span className="verdict-sub mono">
                {evalResult.allowed ? 'Safe for autonomous agent execution' : 'Intercepted by msh guardrail'}
              </span>
            </div>

            {evalResult.violations && evalResult.violations.length > 0 ? (
              <div className="verdict-violations-list">
                {evalResult.violations.map((v: any, vi: number) => (
                  <div key={vi} className="verdict-violation-row">
                    <div className="violation-top">
                      <span className="violation-rule-id mono">{v.rule_id}</span>
                      <span className="violation-action mono">{v.action?.toUpperCase()}</span>
                    </div>
                    <div className="violation-msg">{v.message}</div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="verdict-clean-msg">
                No policy violations detected. This command meets all baseline safety standards.
              </div>
            )}
          </div>
        )}
      </div>

      {/* Active Policies Table */}
      <div className="active-policies-card">
        <div className="policies-table-header">
          <div className="table-title-group">
            <h3 className="policies-table-title">Active Security Policies Matrix</h3>
            <span className="policies-count-badge mono">{filteredPolicies.length} Rules Active</span>
          </div>

          <div className="table-controls">
            <div className="policies-search-wrap">
              <SearchIcon size={13} className="policies-search-icon" />
              <input
                type="text"
                className="policies-search-input mono"
                placeholder="Search rules, messages, patterns..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
            <div className="risk-filter-pills">
              {['all', 'critical', 'high', 'moderate'].map((r) => (
                <button
                  key={r}
                  type="button"
                  className={`risk-filter-btn ${filterRisk === r ? 'active' : ''}`}
                  onClick={() => setFilterRisk(r)}
                >
                  {r.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="policies-table-scroll">
          <table className="policies-table">
            <thead>
              <tr>
                <th className="th-rule">RULE ID</th>
                <th className="th-action">ACTION</th>
                <th className="th-risk">RISK</th>
                <th className="th-pattern">MATCH PATTERN</th>
                <th className="th-desc">DESCRIPTION & REASONING</th>
              </tr>
            </thead>
            <tbody>
              {filteredPolicies.map((p, idx) => (
                <tr key={idx} className="policy-row">
                  <td className="td-rule mono">{p.id}</td>
                  <td className="td-action">
                    <span className={`policy-action-pill ${p.action === 'block' ? 'action-block' : 'action-warn'}`}>
                      {p.action.toUpperCase()}
                    </span>
                  </td>
                  <td className="td-risk">
                    <span className={`guard-risk-pill risk-${p.risk}`}>
                      {p.risk.toUpperCase()}
                    </span>
                  </td>
                  <td className="td-pattern mono" title={p.match}>
                    <code>{p.match}</code>
                  </td>
                  <td className="td-desc">
                    <div className="policy-desc-text">{p.description || p.message}</div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
