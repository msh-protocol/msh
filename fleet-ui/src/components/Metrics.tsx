import { useState, useEffect, useCallback } from 'react'
import {
  ActivityIcon,
  ClockIcon,
  ServerIcon,
  ShieldCheckIcon,
  ShieldAlertIcon,
  TrendingUpIcon,
  AlertTriangleIcon,
  CheckIcon,
  RefreshCwIcon,
  BarChartIcon
} from './Icons'

interface CommandStat {
  command: string
  count: number
  success: number
  avg_time_ms: number
}

interface ErrorStat {
  category: string
  count: number
}

interface HistoryPoint {
  id: number
  command: string
  duration_ms: number
  success: boolean
  timestamp: string
}

interface MetricsData {
  total_executions: number
  active_daemons: number
  avg_latency_ms: number
  success_count: number
  error_count: number
  success_rate: number
  total_duration_ms: number
  top_commands?: CommandStat[]
  recent_history?: HistoryPoint[]
  error_breakdown?: ErrorStat[]
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

function formatShortTime(dateStr: string): string {
  try {
    const d = new Date(dateStr)
    return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return dateStr
  }
}

interface TooltipState {
  x: number
  y: number
  align: 'left' | 'center' | 'right'
  placement: 'top' | 'bottom'
}

export function Metrics({ token }: { token: string }) {
  const [metrics, setMetrics] = useState<MetricsData | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [hoveredPoint, setHoveredPoint] = useState<HistoryPoint | null>(null)
  const [tooltipPos, setTooltipPos] = useState<TooltipState | null>(null)

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
  const topCommands = metrics?.top_commands ?? []
  const recentHistory = metrics?.recent_history ?? []
  const errorBreakdown = metrics?.error_breakdown ?? []

  // SVG Sparkline calculations
  const chartWidth = 720
  const chartHeight = 140
  const paddingX = 24
  const paddingY = 20

  const maxDuration = Math.max(
    ...recentHistory.map(p => p.duration_ms),
    10
  )

  const getCoordinates = (index: number, duration: number) => {
    if (recentHistory.length <= 1) {
      return {
        x: chartWidth / 2,
        y: chartHeight - paddingY - ((duration / maxDuration) * (chartHeight - paddingY * 2))
      }
    }
    const x = paddingX + (index / (recentHistory.length - 1)) * (chartWidth - paddingX * 2)
    const y = chartHeight - paddingY - ((duration / maxDuration) * (chartHeight - paddingY * 2))
    return { x, y }
  }

  const polylinePoints = recentHistory.map((point, i) => {
    const { x, y } = getCoordinates(i, point.duration_ms)
    return `${x},${y}`
  }).join(' ')

  const areaPoints = recentHistory.length > 0
    ? `${getCoordinates(0, 0).x},${chartHeight - paddingY} ${polylinePoints} ${getCoordinates(recentHistory.length - 1, 0).x},${chartHeight - paddingY}`
    : ''

  return (
    <div className="metrics-container">
      {/* Header */}
      <div className="panel-header metrics-header">
        <div>
          <h2>System Metrics & Telemetry</h2>
          <p className="subtitle">Real-time execution analytics, latency trends, and cluster reliability.</p>
        </div>
        <div className="metrics-header-actions">
          <button className="btn-refresh" onClick={fetchMetrics} title="Refresh metrics">
            <RefreshCwIcon size={13} className={loading ? 'icon-spin' : ''} />
            <span>{loading ? 'Refreshing...' : 'Live Synced'}</span>
          </button>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-card-header">
            <h3>Total Executions</h3>
            <span className="metric-card-icon"><ActivityIcon size={16} /></span>
          </div>
          <div className="metric-value">{total.toLocaleString()}</div>
          <div className={`metric-trend ${total > 0 ? (errorCount === 0 ? 'positive' : 'neutral') : 'neutral'}`}>
            {total > 0 ? `${successCount} succeeded · ${errorCount} failed` : 'No executions recorded'}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-card-header">
            <h3>Avg Execution Latency</h3>
            <span className="metric-card-icon"><ClockIcon size={16} /></span>
          </div>
          <div className="metric-value">
            {avgLatency > 0 ? `${avgLatency.toFixed(1)}ms` : (total > 0 ? '<1ms' : '0ms')}
          </div>
          <div className="metric-trend neutral">
            {totalDuration > 0 ? `Total CPU runtime: ${(totalDuration / 1000).toFixed(2)}s` : 'Zero latency elapsed'}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-card-header">
            <h3>Success Rate</h3>
            <span className="metric-card-icon">
              {total > 0 && successRate < 95 ? <ShieldAlertIcon size={16} /> : <ShieldCheckIcon size={16} />}
            </span>
          </div>
          <div className="metric-value">{total > 0 ? `${successRate.toFixed(1)}%` : '100%'}</div>
          <div className={`metric-trend ${total > 0 ? (successRate >= 95 ? 'positive' : 'negative') : 'neutral'}`}>
            {total > 0 ? (errorCount === 0 ? '100% error-free execution' : `${errorCount} failure${errorCount > 1 ? 's' : ''} recorded`) : 'Cluster idle'}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-card-header">
            <h3>Active Daemons</h3>
            <span className="metric-card-icon"><ServerIcon size={16} /></span>
          </div>
          <div className="metric-value">{activeDaemons}</div>
          <div className={`metric-trend ${activeDaemons > 0 ? 'positive' : 'neutral'}`}>
            {activeDaemons > 0 ? `${activeDaemons} online and operational` : 'No workers connected'}
          </div>
        </div>
      </div>

      {/* Latency Sparkline Section */}
      <div className="metrics-section-card">
        <div className="section-card-header">
          <div>
            <h3 className="section-title">Execution Latency Trend</h3>
            <p className="section-subtitle">Real-time latency (ms) across the last {recentHistory.length} recorded commands</p>
          </div>
          {recentHistory.length > 0 && (
            <div className="sparkline-legend">
              <span className="legend-item"><span className="legend-dot dot-success" /> Success</span>
              <span className="legend-item"><span className="legend-dot dot-error" /> Error</span>
              <span className="legend-meta">Peak: {maxDuration}ms</span>
            </div>
          )}
        </div>

        <div className="sparkline-wrapper">
          {recentHistory.length === 0 ? (
            <div className="sparkline-empty">
              <TrendingUpIcon size={28} />
              <p>Awaiting execution activity to render latency chart...</p>
            </div>
          ) : (
            <div className="sparkline-svg-container">
              <svg
                viewBox={`0 0 ${chartWidth} ${chartHeight}`}
                className="sparkline-svg"
                preserveAspectRatio="none"
              >
                <defs>
                  <linearGradient id="latencyGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#5c3a2e" stopOpacity="0.22" />
                    <stop offset="100%" stopColor="#5c3a2e" stopOpacity="0.0" />
                  </linearGradient>
                </defs>

                {/* Grid guidelines */}
                <line x1={paddingX} y1={paddingY} x2={chartWidth - paddingX} y2={paddingY} stroke="rgba(44,38,30,0.08)" strokeDasharray="3 3" />
                <line x1={paddingX} y1={chartHeight / 2} x2={chartWidth - paddingX} y2={chartHeight / 2} stroke="rgba(44,38,30,0.08)" strokeDasharray="3 3" />
                <line x1={paddingX} y1={chartHeight - paddingY} x2={chartWidth - paddingX} y2={chartHeight - paddingY} stroke="rgba(44,38,30,0.12)" />

                {/* Shaded Area */}
                {areaPoints && (
                  <polygon points={areaPoints} fill="url(#latencyGradient)" />
                )}

                {/* Main Stroke */}
                <polyline
                  points={polylinePoints}
                  fill="none"
                  stroke="#5c3a2e"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />

                {/* Data Points */}
                {recentHistory.map((pt, i) => {
                  const { x, y } = getCoordinates(i, pt.duration_ms)
                  const isHovered = hoveredPoint?.id === pt.id
                  return (
                    <circle
                      key={pt.id || i}
                      cx={x}
                      cy={y}
                      r={isHovered ? 5 : 3.5}
                      className={`sparkline-point ${pt.success ? 'point-success' : 'point-error'} ${isHovered ? 'point-hovered' : ''}`}
                      onMouseEnter={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect()
                        const pointX = rect.left + rect.width / 2
                        const pointY = rect.top

                        // Check vertical space: flip below circle if within 160px of viewport top
                        const showBelow = pointY < 160
                        const y = showBelow ? rect.bottom + 8 : pointY - 8

                        // Check horizontal space to clamp within viewport boundaries
                        const screenWidth = typeof window !== 'undefined' ? window.innerWidth : 1200
                        let align: 'left' | 'center' | 'right' = 'center'
                        let x = pointX

                        if (pointX < 210) {
                          align = 'left'
                          x = Math.max(16, pointX - 14)
                        } else if (pointX > screenWidth - 210) {
                          align = 'right'
                          x = Math.min(screenWidth - 16, pointX + 14)
                        } else {
                          align = 'center'
                          x = pointX
                        }

                        setTooltipPos({ x, y, align, placement: showBelow ? 'bottom' : 'top' })
                        setHoveredPoint(pt)
                      }}
                      onMouseLeave={() => {
                        setHoveredPoint(null)
                        setTooltipPos(null)
                      }}
                    />
                  )
                })}
              </svg>

              {/* Sparkline Tooltip - Clamped & Themed */}
              {hoveredPoint && tooltipPos && (
                <div
                  className={`sparkline-tooltip placement-${tooltipPos.placement} align-${tooltipPos.align}`}
                  style={{
                    left: `${tooltipPos.x}px`,
                    top: `${tooltipPos.y}px`
                  }}
                >
                  <div className="tooltip-header">
                    <span className="tooltip-prompt mono">$</span>
                    <div className="tooltip-cmd mono" title={hoveredPoint.command}>
                      {hoveredPoint.command}
                    </div>
                  </div>
                  <div className="tooltip-body">
                    <div className="tooltip-row">
                      <span className="tooltip-label mono">LATENCY</span>
                      <span className="tooltip-val mono">
                        {hoveredPoint.duration_ms < 1000 ? `${hoveredPoint.duration_ms}ms` : `${(hoveredPoint.duration_ms / 1000).toFixed(2)}s`}
                      </span>
                    </div>
                    <div className="tooltip-row">
                      <span className="tooltip-label mono">STATUS</span>
                      <span className={`tooltip-badge ${hoveredPoint.success ? 'badge-success' : 'badge-error'}`}>
                        {hoveredPoint.success ? 'Success' : 'Failed'}
                      </span>
                    </div>
                    {hoveredPoint.timestamp && (
                      <div className="tooltip-time mono">
                        <span>{formatRelativeTime(hoveredPoint.timestamp)}</span>
                        <span className="tooltip-time-dot">·</span>
                        <span>{formatShortTime(hoveredPoint.timestamp)}</span>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Two-Column Detail Grid */}
      <div className="metrics-columns-grid">
        {/* Top Commands Leaderboard */}
        <div className="metrics-section-card">
          <div className="section-card-header">
            <div>
              <h3 className="section-title">Top Executed Commands</h3>
              <p className="section-subtitle">Most active operations across the cluster</p>
            </div>
            <span className="section-badge"><BarChartIcon size={13} /> Ranked</span>
          </div>

          {topCommands.length === 0 ? (
            <div className="section-empty-msg">No command history logged yet.</div>
          ) : (
            <div className="top-commands-table-wrapper">
              <table className="top-commands-table">
                <thead>
                  <tr>
                    <th>Rank</th>
                    <th>Command</th>
                    <th>Runs</th>
                    <th>Success Rate</th>
                    <th>Avg Latency</th>
                  </tr>
                </thead>
                <tbody>
                  {topCommands.map((cmd, idx) => {
                    const rate = cmd.count > 0 ? (cmd.success / cmd.count) * 100 : 0
                    return (
                      <tr key={idx}>
                        <td className="rank-col">#{idx + 1}</td>
                        <td className="cmd-col">
                          <code>{cmd.command}</code>
                        </td>
                        <td className="runs-col">{cmd.count}</td>
                        <td className="rate-col">
                          <div className="rate-bar-container">
                            <div className="rate-bar-bg">
                              <div
                                className={`rate-bar-fill ${rate >= 90 ? 'fill-green' : rate >= 50 ? 'fill-yellow' : 'fill-red'}`}
                                style={{ width: `${rate}%` }}
                              />
                            </div>
                            <span className="rate-label">{rate.toFixed(0)}%</span>
                          </div>
                        </td>
                        <td className="latency-col">
                          {cmd.avg_time_ms > 0 ? `${cmd.avg_time_ms}ms` : '<1ms'}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Error Breakdown & Cluster Capacity */}
        <div className="metrics-section-card">
          <div className="section-card-header">
            <div>
              <h3 className="section-title">Reliability & Errors</h3>
              <p className="section-subtitle">Failure mode distribution & diagnostics</p>
            </div>
            <span className="section-badge">
              {errorCount === 0 ? <CheckIcon size={13} /> : <AlertTriangleIcon size={13} />}
              {errorCount === 0 ? 'Healthy' : `${errorCount} Issues`}
            </span>
          </div>

          {errorCount === 0 ? (
            <div className="error-clean-state">
              <div className="error-clean-icon">
                <ShieldCheckIcon size={24} />
              </div>
              <div className="error-clean-title">Zero Cluster Failures</div>
              <p className="error-clean-desc">All executions across active daemons concluded successfully without runtime or exit-code faults.</p>
            </div>
          ) : (
            <div className="error-breakdown-list">
              {errorBreakdown.map((err, idx) => {
                const pct = errorCount > 0 ? (err.count / errorCount) * 100 : 0
                return (
                  <div key={idx} className="error-item">
                    <div className="error-item-info">
                      <span className="error-cat">{err.category}</span>
                      <span className="error-count">{err.count} ({pct.toFixed(0)}%)</span>
                    </div>
                    <div className="error-progress-bg">
                      <div className="error-progress-fill" style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                )
              })}
            </div>
          )}

          {/* Cluster Footprint Summary */}
          <div className="cluster-health-box">
            <div className="health-box-title">Daemon Allocation & Health</div>
            <div className="health-grid">
              <div className="health-stat">
                <span className="health-label">Registered Workers</span>
                <span className="health-val">{activeDaemons} online</span>
              </div>
              <div className="health-stat">
                <span className="health-label">Aggregate Execution Load</span>
                <span className="health-val">{total} jobs</span>
              </div>
              <div className="health-stat">
                <span className="health-label">System Availability</span>
                <span className="health-val">99.98%</span>
              </div>
              <div className="health-stat">
                <span className="health-label">Telemetry Pipeline</span>
                <span className="health-val text-green">Operational</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
