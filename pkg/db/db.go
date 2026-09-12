package db

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// MetricsData aggregates execution statistics computed from database records.
type MetricsData struct {
	TotalExecutions int     `json:"total_executions"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	SuccessCount    int     `json:"success_count"`
	ErrorCount      int     `json:"error_count"`
	SuccessRate     float64 `json:"success_rate"`
	TotalDurationMs int64   `json:"total_duration_ms"`
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
		return MetricsData{}, nil
	}

	var totalDuration int64
	var successCount int
	var errorCount int

	for _, r := range records {
		totalDuration += r.DurationMs
		if r.Status == "success" || (r.ExitCode == 0 && r.Status != "error" && r.Status != "timeout" && r.Status != "blocked") {
			successCount++
		} else {
			errorCount++
		}
	}

	avgLatency := float64(totalDuration) / float64(total)
	successRate := (float64(successCount) / float64(total)) * 100.0

	return MetricsData{
		TotalExecutions: total,
		AvgLatencyMs:    avgLatency,
		SuccessCount:    successCount,
		ErrorCount:      errorCount,
		SuccessRate:     successRate,
		TotalDurationMs: totalDuration,
	}, nil
}
