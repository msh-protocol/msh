import { useEffect } from 'react'
import { XIcon } from './Icons'
import { MshLogo } from './MshLogo'
import './ShortcutsModal.css'

interface ShortcutsModalProps {
  isOpen: boolean
  onClose: () => void
}

interface ShortcutItem {
  keys: string[]
  desc: string
  category: 'Navigation' | 'Actions & Search' | 'Terminal Controls'
}

const SHORTCUTS: ShortcutItem[] = [
  { keys: ['1'], desc: 'Switch to Live Swarm Grid tab', category: 'Navigation' },
  { keys: ['2'], desc: 'Switch to Execution History tab', category: 'Navigation' },
  { keys: ['3'], desc: 'Switch to Cluster Metrics tab', category: 'Navigation' },
  { keys: ['n'], desc: 'Toggle Connected Swarm Daemons drawer', category: 'Navigation' },
  { keys: ['/'], desc: 'Focus Swarm Broadcast input (or History search)', category: 'Actions & Search' },
  { keys: ['r'], desc: 'Refresh telemetry metrics & execution ledger', category: 'Actions & Search' },
  { keys: ['?'], desc: 'Toggle this keyboard shortcuts cheatsheet', category: 'Actions & Search' },
  { keys: ['↑', '↓'], desc: 'Cycle through previously executed command history', category: 'Terminal Controls' },
  { keys: ['Esc'], desc: 'Close any active drawer, modal, or blur inputs', category: 'Terminal Controls' },
]

export function ShortcutsModal({ isOpen, onClose }: ShortcutsModalProps) {
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

  const categories = ['Navigation', 'Actions & Search', 'Terminal Controls'] as const

  return (
    <div className="shortcuts-modal-backdrop" onClick={onClose}>
      <div className="shortcuts-modal" onClick={e => e.stopPropagation()}>
        <div className="shortcuts-modal-header">
          <div className="shortcuts-header-left">
            <span className="shortcuts-icon">
              <MshLogo size={22} />
            </span>
            <div>
              <h3 className="shortcuts-title">Keyboard Navigation & Shortcuts</h3>
              <span className="shortcuts-subtitle">
                High-agency cluster controls for keyboard-first operators
              </span>
            </div>
          </div>
          <button
            type="button"
            className="shortcuts-close-btn"
            onClick={onClose}
            title="Close modal (Esc)"
          >
            <XIcon size={14} />
            <span className="kbd-shortcut mono">ESC</span>
          </button>
        </div>

        <div className="shortcuts-modal-body">
          {categories.map(cat => {
            const items = SHORTCUTS.filter(s => s.category === cat)
            return (
              <div key={cat} className="shortcuts-section">
                <h4 className="shortcuts-section-title">{cat}</h4>
                <div className="shortcuts-list">
                  {items.map((item, idx) => (
                    <div key={idx} className="shortcut-row">
                      <span className="shortcut-desc">{item.desc}</span>
                      <div className="shortcut-keys">
                        {item.keys.map((k, kIdx) => (
                          <kbd key={kIdx} className="kbd-cap mono">
                            {k}
                          </kbd>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>

        <div className="shortcuts-modal-footer">
          <span className="shortcuts-tip">
            Tip: Shortcuts are active across all views when not focused in a text field.
          </span>
        </div>
      </div>
    </div>
  )
}
