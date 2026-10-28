package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DaemonState struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Cwd       string    `json:"cwd"`
	Pid       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"` // running, exited, killed
	ExitCode  int       `json:"exit_code,omitempty"`
}

type Manager struct {
	baseDir string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Join(home, ".msh", "daemons")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &Manager{baseDir: baseDir}, nil
}

func (m *Manager) Start(commandStr, cwd string, env map[string]string) (*DaemonState, error) {
	id := uuid.New().String()
	daemonDir := filepath.Join(m.baseDir, id)
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		return nil, err
	}

	if cwd == "" {
		if cur, err := os.Getwd(); err == nil {
			cwd = cur
		}
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", commandStr)
	} else {
		cmd = exec.Command("sh", "-c", commandStr)
	}
	setDetached(cmd)
	cmd.Dir = cwd

	// Always inherit current process environment, then layer requested variables
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Redirect output to a log file
	logFile, err := os.OpenFile(filepath.Join(daemonDir, "output.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}
	// Close our parent handle so child owns the inherited descriptor
	logFile.Close()

	state := &DaemonState{
		ID:        id,
		Command:   commandStr,
		Cwd:       cwd,
		Pid:       cmd.Process.Pid,
		StartTime: time.Now(),
		Status:    "running",
	}

	if err := m.saveState(state); err != nil {
		return nil, err
	}

	// Release the process so it keeps running in the background
	if err := cmd.Process.Release(); err != nil {
		return nil, err
	}

	return state, nil
}

func (m *Manager) saveState(state *DaemonState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.baseDir, state.ID, "state.json"), data, 0600)
}

func (m *Manager) GetState(id string) (*DaemonState, error) {
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return nil, fmt.Errorf("invalid daemon id")
	}
	data, err := os.ReadFile(filepath.Join(m.baseDir, id, "state.json"))
	if err != nil {
		return nil, err
	}
	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Check if process is still alive using platform-accurate inspection
	if state.Status == "running" {
		alive, exitCode := checkProcessLiveness(state.Pid)
		if !alive {
			state.Status = "exited"
			state.ExitCode = exitCode
			_ = m.saveState(&state)
		}
	}

	return &state, nil
}

func (m *Manager) Kill(id string) error {
	state, err := m.GetState(id)
	if err != nil {
		return err
	}
	process, err := os.FindProcess(state.Pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// On Windows, process.Kill() only kills the parent cmd.exe, leaving child Node/Go processes alive.
		// We must use taskkill /T to kill the entire tree.
		exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", state.Pid)).Run()
	} else {
		if err := process.Kill(); err != nil {
			return err
		}
	}
	state.Status = "killed"
	m.saveState(state)
	return nil
}

func (m *Manager) ReadLogs(id string, maxLines int) (string, error) {
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid daemon id")
	}
	state, _ := m.GetState(id)

	data, err := os.ReadFile(filepath.Join(m.baseDir, id, "output.log"))
	if err != nil {
		if state != nil && state.Status == "exited" {
			return fmt.Sprintf("[msh:daemon] process %d exited with code %d (no output file was generated)\n", state.Pid, state.ExitCode), nil
		}
		return "", err
	}

	content := strings.TrimSpace(string(data))
	if content == "" && state != nil && state.Status == "exited" {
		return fmt.Sprintf("[msh:daemon] process %d exited with code %d (no output was written to output.log)\n", state.Pid, state.ExitCode), nil
	}

	return string(data), nil
}
