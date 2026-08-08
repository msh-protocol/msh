export function Metrics() {
  return (
    <div className="metrics-container">
      <h2>System Metrics</h2>
      <p className="subtitle">Real-time performance and cost analytics.</p>
      
      <div className="metrics-grid">
        <div className="metric-card">
          <h3>Total Executions</h3>
          <div className="metric-value">1,492</div>
          <div className="metric-trend positive">+12% this week</div>
        </div>
        <div className="metric-card">
          <h3>Avg Execution Latency</h3>
          <div className="metric-value">34ms</div>
          <div className="metric-trend positive">-5ms this week</div>
        </div>
        <div className="metric-card">
          <h3>Active Daemons</h3>
          <div className="metric-value">5</div>
          <div className="metric-trend neutral">Stable</div>
        </div>
        <div className="metric-card">
          <h3>Est. Token Usage</h3>
          <div className="metric-value">4.2M</div>
          <div className="metric-trend negative">+800k this week</div>
        </div>
      </div>
    </div>
  )
}
