package db

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// DB represents a flat JSON file database.
type DB struct {
	path string
	mu   sync.RWMutex
}

// ExecutionRecord represents a single flattened row in the history table.
type ExecutionRecord struct {
	ID         int    `json:"ID"`
	Timestamp  string `json:"Timestamp"`
	SessionID  string `json:"SessionID"`
	Command    string `json:"Command"`
	Status     string `json:"Status"`
	ExitCode   int    `json:"ExitCode"`
	DurationMs int64  `json:"DurationMs"`
	ReqJSON    string `json:"ReqJSON"`
	RespJSON   string `json:"RespJSON"`
}

// InitDB initializes the JSON database at ~/.msh/fleet.json
func InitDB() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	dbDir := filepath.Join(home, ".msh")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "fleet.json")
	
	// Create if not exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		os.WriteFile(dbPath, []byte("[]"), 0644)
	}

	return &DB{path: dbPath}, nil
}

func (db *DB) load() ([]ExecutionRecord, error) {
	data, err := os.ReadFile(db.path)
	if err != nil {
		return nil, err
	}
	var records []ExecutionRecord
	if len(data) > 0 {
		json.Unmarshal(data, &records)
	}
	return records, nil
}

func (db *DB) save(records []ExecutionRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.path, data, 0644)
}

// SaveExecution serializes and saves an ExecRequest and ExecResponse to the database.
func (db *DB) SaveExecution(req protocol.ExecRequest, resp protocol.ExecResponse) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	records, err := db.load()
	if err != nil {
		return err
	}

	reqBytes, _ := json.Marshal(req)
	respBytes, err := resp.ToJSON()
	if err != nil {
		return err
	}

	record := ExecutionRecord{
		ID:         len(records) + 1,
		Timestamp:  time.Now().Format(time.RFC3339),
		SessionID:  req.SessionID,
		Command:    req.Command,
		Status:     string(resp.Status),
		ExitCode:   resp.ExitCode,
		DurationMs: resp.DurationMs,
		ReqJSON:    string(reqBytes),
		RespJSON:   string(respBytes),
	}

	records = append([]ExecutionRecord{record}, records...) // Prepend so newest is first

	if err := db.save(records); err != nil {
		log.Printf("Failed to save execution to db: %v", err)
		return err
	}
	return nil
}

// GetExecutions returns a list of recent execution records, ordered by newest first.
func (db *DB) GetExecutions(limit, offset int) ([]ExecutionRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	records, err := db.load()
	if err != nil {
		return nil, err
	}

	if offset >= len(records) {
		return []ExecutionRecord{}, nil
	}

	end := offset + limit
	if end > len(records) {
		end = len(records)
	}

	return records[offset:end], nil
}

// Count returns the total number of execution records.
func (db *DB) Count() (int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	records, err := db.load()
	if err != nil {
		return 0, err
	}
	return len(records), nil
}

// DeleteExecution removes a single execution record by ID.
func (db *DB) DeleteExecution(id int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	records, err := db.load()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]ExecutionRecord, 0, len(records))
	for _, r := range records {
		if r.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}

	if !found {
		return fmt.Errorf("record #%d not found", id)
	}

	return db.save(filtered)
}

// ClearExecutions clears all stored execution history records.
func (db *DB) ClearExecutions() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.save([]ExecutionRecord{})
}

// CommandStat aggregates execution frequency and latency for top commands.
type CommandStat struct {
	Command   string  `json:"command"`
	Count     int     `json:"count"`
	Success   int     `json:"success"`
	AvgTimeMs float64 `json:"avg_time_ms"`
}

// ErrorStat categorizes failures.
type ErrorStat struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// HistoryPoint captures recent individual execution telemetry for timeline/sparkline charts.
type HistoryPoint struct {
	ID         int    `json:"id"`
	Command    string `json:"command"`
	DurationMs int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
	Timestamp  string `json:"timestamp"`
}

// MetricsData aggregates execution statistics computed from database records.
type MetricsData struct {
	TotalExecutions int            `json:"total_executions"`
	AvgLatencyMs    float64        `json:"avg_latency_ms"`
	SuccessCount    int            `json:"success_count"`
	ErrorCount      int            `json:"error_count"`
	SuccessRate     float64        `json:"success_rate"`
	TotalDurationMs int64          `json:"total_duration_ms"`
	TopCommands     []CommandStat  `json:"top_commands"`
	RecentHistory   []HistoryPoint `json:"recent_history"`
	ErrorBreakdown  []ErrorStat    `json:"error_breakdown"`
}

// GetMetrics returns aggregated statistics from all stored execution records.
func (db *DB) GetMetrics() (MetricsData, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	records, err := db.load()
	if err != nil {
		return MetricsData{}, err
	}

	total := len(records)
	if total == 0 {
		return MetricsData{
			TopCommands:    []CommandStat{},
			RecentHistory:  []HistoryPoint{},
			ErrorBreakdown: []ErrorStat{},
		}, nil
	}

	var totalDuration int64
	var successCount int
	var errorCount int

	type cmdAgg struct {
		count         int
		success       int
		totalDuration int64
	}
	cmdMap := make(map[string]*cmdAgg)
	errorCatMap := make(map[string]int)

	for _, r := range records {
		totalDuration += r.DurationMs
		isSuccess := r.Status == "success" || (r.ExitCode == 0 && r.Status != "error" && r.Status != "timeout" && r.Status != "blocked")
		if isSuccess {
			successCount++
		} else {
			errorCount++
			cat := fmt.Sprintf("Exit Code %d", r.ExitCode)
			if r.Status == "timeout" {
				cat = "Timeout"
			} else if r.Status == "blocked" {
				cat = "Security Blocked"
			} else if r.Status == "error" {
				cat = "Execution Error"
			}
			errorCatMap[cat]++
		}

		cmdName := strings.TrimSpace(r.Command)
		parts := strings.Fields(cmdName)
		displayCmd := cmdName
		if len(parts) > 3 {
			displayCmd = strings.Join(parts[:3], " ") + "..."
		}
		if displayCmd == "" {
			displayCmd = "(empty)"
		}

		agg, exists := cmdMap[displayCmd]
		if !exists {
			agg = &cmdAgg{}
			cmdMap[displayCmd] = agg
		}
		agg.count++
		agg.totalDuration += r.DurationMs
		if isSuccess {
			agg.success++
		}
	}

	avgLatency := float64(totalDuration) / float64(total)
	successRate := (float64(successCount) / float64(total)) * 100.0

	// Top commands
	var topCmds []CommandStat
	for cmd, agg := range cmdMap {
		avgTime := float64(agg.totalDuration) / float64(agg.count)
		topCmds = append(topCmds, CommandStat{
			Command:   cmd,
			Count:     agg.count,
			Success:   agg.success,
			AvgTimeMs: math.Round(avgTime*10) / 10,
		})
	}
	sort.Slice(topCmds, func(i, j int) bool {
		if topCmds[i].Count == topCmds[j].Count {
			return topCmds[i].AvgTimeMs < topCmds[j].AvgTimeMs
		}
		return topCmds[i].Count > topCmds[j].Count
	})
	if len(topCmds) > 5 {
		topCmds = topCmds[:5]
	}

	// Error breakdown
	var errorBreakdown []ErrorStat
	for cat, cnt := range errorCatMap {
		errorBreakdown = append(errorBreakdown, ErrorStat{
			Category: cat,
			Count:    cnt,
		})
	}
	sort.Slice(errorBreakdown, func(i, j int) bool {
		return errorBreakdown[i].Count > errorBreakdown[j].Count
	})

	// Recent history points (up to last 30 in chronological order)
	historyLimit := 30
	startIdx := 0
	if total > historyLimit {
		startIdx = total - historyLimit
	}
	recentHistory := make([]HistoryPoint, 0, total-startIdx)
	for i := startIdx; i < total; i++ {
		rec := records[i]
		isSuccess := rec.Status == "success" || (rec.ExitCode == 0 && rec.Status != "error" && rec.Status != "timeout" && rec.Status != "blocked")
		recentHistory = append(recentHistory, HistoryPoint{
			ID:         rec.ID,
			Command:    rec.Command,
			DurationMs: rec.DurationMs,
			Success:    isSuccess,
			Timestamp:  rec.Timestamp,
		})
	}

	return MetricsData{
		TotalExecutions: total,
		AvgLatencyMs:    avgLatency,
		SuccessCount:    successCount,
		ErrorCount:      errorCount,
		SuccessRate:     successRate,
		TotalDurationMs: totalDuration,
		TopCommands:     topCmds,
		RecentHistory:   recentHistory,
		ErrorBreakdown:  errorBreakdown,
	}, nil
}
