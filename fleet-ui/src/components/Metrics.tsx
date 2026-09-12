import { useState, useEffect, useCallback } from 'react'

interface MetricsData {
  total_executions: number
  active_daemons: number
  avg_latency_ms: number
  success_count: number
  error_count: number
  success_rate: number
  total_duration_ms: number
}

export function Metrics({ token }: { token: string }) {
  const [metrics, setMetrics] = useState<MetricsData | null>(null)
  const [loading, setLoading] = useState<boolean>(true)

  const fetchMetrics = useCallback(async () => {
    try {
      const res = await fetch('/api/metrics', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      if (res.ok) {
        const data = await res.json()
        setMetrics(data)
      }
    } catch (err) {
      console.error("Failed to fetch metrics", err)
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    fetchMetrics()
    const interval = setInterval(fetchMetrics, 3000)
    return () => clearInterval(interval)
  }, [fetchMetrics])

  const total = metrics?.total_executions ?? 0
  const activeDaemons = metrics?.active_daemons ?? 0
  const avgLatency = metrics?.avg_latency_ms ?? 0
  const successRate = metrics?.success_rate ?? 0
  const successCount = metrics?.success_count ?? 0
  const errorCount = metrics?.error_count ?? 0
  const totalDuration = metrics?.total_duration_ms ?? 0

  return (
    <div className="metrics-container">
      <div className="panel-header" style={{ marginBottom: '24px', padding: 0, border: 'none' }}>
        <div>
          <h2>System Metrics</h2>
          <p className="subtitle" style={{ margin: '4px 0 0' }}>Live execution analytics computed across registered daemons.</p>
        </div>
        {loading && <span className="badge-count">Refreshing...</span>}
      </div>
      
      <div className="metrics-grid">
        <div className="metric-card">
          <h3>Total Executions</h3>
          <div className="metric-value">{total.toLocaleString()}</div>
          <div className={`metric-trend ${total > 0 ? (errorCount === 0 ? 'positive' : 'neutral') : 'neutral'}`}>
            {total > 0 ? `${successCount} succeeded · ${errorCount} failed` : 'No executions recorded yet'}
          </div>
        </div>

        <div className="metric-card">
          <h3>Avg Execution Latency</h3>
          <div className="metric-value">{avgLatency > 0 ? `${avgLatency}ms` : (total > 0 ? '<1ms' : '0ms')}</div>
          <div className="metric-trend neutral">
            {totalDuration > 0 ? `Total runtime: ${(totalDuration / 1000).toFixed(2)}s` : 'No latency recorded'}
          </div>
        </div>

        <div className="metric-card">
          <h3>Active Daemons</h3>
          <div className="metric-value">{activeDaemons}</div>
          <div className={`metric-trend ${activeDaemons > 0 ? 'positive' : 'neutral'}`}>
            {activeDaemons > 0 ? `${activeDaemons} connected worker${activeDaemons > 1 ? 's' : ''}` : '0 workers online'}
          </div>
        </div>

        <div className="metric-card">
          <h3>Success Rate</h3>
          <div className="metric-value">{total > 0 ? `${successRate.toFixed(1)}%` : 'N/A'}</div>
          <div className={`metric-trend ${total > 0 ? (successRate >= 90 ? 'positive' : 'negative') : 'neutral'}`}>
            {total > 0 ? (errorCount === 0 ? '100% error-free' : `${errorCount} run failure${errorCount > 1 ? 's' : ''}`) : 'Awaiting executions'}
          </div>
        </div>
      </div>

      <div className="metrics-summary-card" style={{
        marginTop: '24px',
        padding: '20px',
        border: '1px solid var(--border-color)',
        borderRadius: '8px',
        backgroundColor: 'var(--bg-panel)'
      }}>
        <h3 style={{ fontSize: '14px', fontWeight: 600, marginBottom: '12px' }}>Fleet Overview</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '16px', fontSize: '13px', color: 'var(--text-secondary)' }}>
          <div>
            <span style={{ display: 'block', color: 'var(--text-primary)', fontWeight: 500 }}>Connected Agents</span>
            {activeDaemons > 0 ? `${activeDaemons} online and ready` : 'No agents currently connected'}
          </div>
          <div>
            <span style={{ display: 'block', color: 'var(--text-primary)', fontWeight: 500 }}>Execution Health</span>
            {total > 0 ? `${successRate.toFixed(1)}% reliability rate` : 'Healthy (idle)'}
          </div>
          <div>
            <span style={{ display: 'block', color: 'var(--text-primary)', fontWeight: 500 }}>Total Compute Time</span>
            {totalDuration > 0 ? `${totalDuration}ms total CPU elapsed` : '0ms elapsed'}
          </div>
        </div>
      </div>
    </div>
  )
}
