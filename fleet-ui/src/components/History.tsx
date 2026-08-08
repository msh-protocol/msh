import { useState, useEffect } from 'react'

interface ExecutionRecord {
  ID: number
  Timestamp: string
  SessionID: string
  Command: string
  Status: string
  ExitCode: number
  DurationMs: number
}

export function History({ token }: { token: string }) {
  const [records, setRecords] = useState<ExecutionRecord[]>([])

  useEffect(() => {
    const fetchHistory = async () => {
      try {
        const res = await fetch('/api/history', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (res.ok) {
          const data = await res.json()
          setRecords(data || [])
        }
      } catch (err) {
        console.error("Failed to fetch history", err)
      }
    }
    fetchHistory()
    const interval = setInterval(fetchHistory, 5000)
    return () => clearInterval(interval)
  }, [token])

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
              <tr key={r.ID}>
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
            ))}
            {records.length === 0 && (
              <tr>
                <td colSpan={6} className="empty-state">No execution history found.</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
