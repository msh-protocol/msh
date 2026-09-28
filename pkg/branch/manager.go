package branch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
)

var (
	ErrNotGitRepo       = errors.New("workspace is not inside a git repository")
	ErrBranchExists     = errors.New("shadow branch already exists")
	ErrBranchNotFound   = errors.New("shadow branch not found")
	ErrInvalidName      = errors.New("invalid branch name: must contain only letters, numbers, hyphens, and underscores")
	validBranchNameRe   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// BranchInfo holds metadata regarding an active or historical shadow worktree.
type BranchInfo struct {
	Name          string    `json:"name"`
	GitBranch     string    `json:"git_branch"`
	Path          string    `json:"path"`
	BaseRef       string    `json:"base_ref"`
	BaseCommit    string    `json:"base_commit"`
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status"` // active, merged, aborted
	Description   string    `json:"description,omitempty"`
	RunsCount     int       `json:"runs_count"`
	ModifiedFiles []string  `json:"modified_files"`
}

// PrunePayload is emitted when an exploratory hypothesis is aborted (arXiv:2608.03836).
// It informs LLM harnesses which conversation turns or speculative tool results to prune.
type PrunePayload struct {
	Branch            string    `json:"branch"`
	Status            string    `json:"status"`
	AbortedAt         time.Time `json:"aborted_at"`
	ContextPruneTurns int       `json:"context_prune_turns"`
	Summary           string    `json:"summary"`
	FilesCleaned      []string  `json:"files_cleaned"`
}

// Manager orchestrates sub-second speculative git worktrees for autonomous agents.
type Manager struct {
	WorkspaceDir string
	ShadowDir    string
}

// NewManager initializes a branch manager anchored to a workspace.
func NewManager(workspaceDir string) (*Manager, error) {
	if workspaceDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workspaceDir = cwd
	}

	absWorkspace, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, err
	}

	// Verify git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = absWorkspace
	if err := cmd.Run(); err != nil {
		return nil, ErrNotGitRepo
	}

	shadowDir := filepath.Join(absWorkspace, ".msh", "branches")
	if err := os.MkdirAll(shadowDir, 0755); err != nil {
		return nil, err
	}

	return &Manager{
		WorkspaceDir: absWorkspace,
		ShadowDir:    shadowDir,
	}, nil
}

// Create provisions an isolated shadow git worktree in milliseconds.
func (m *Manager) Create(name, baseRef, desc string) (*BranchInfo, error) {
	name = strings.TrimSpace(name)
	if !validBranchNameRe.MatchString(name) {
		return nil, ErrInvalidName
	}

	targetPath := filepath.Join(m.ShadowDir, name)
	if _, err := os.Stat(targetPath); err == nil {
		return nil, ErrBranchExists
	}

	if baseRef == "" {
		baseRef = "HEAD"
	}

	// Resolve base commit hash
	revCmd := exec.Command("git", "rev-parse", baseRef)
	revCmd.Dir = m.WorkspaceDir
	commitBytes, err := revCmd.Output()
	baseCommit := strings.TrimSpace(string(commitBytes))
	if err != nil {
		baseCommit = baseRef
	}

	gitBranch := fmt.Sprintf("msh-shadow-%s", name)

	// Clean up stale git branch reference if lingering
	_ = exec.Command("git", "branch", "-D", gitBranch).Run()

	// git worktree add -b <gitBranch> <targetPath> <baseRef>
	addCmd := exec.Command("git", "worktree", "add", "-b", gitBranch, targetPath, baseRef)
	addCmd.Dir = m.WorkspaceDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	info := &BranchInfo{
		Name:          name,
		GitBranch:     gitBranch,
		Path:          targetPath,
		BaseRef:       baseRef,
		BaseCommit:    baseCommit,
		CreatedAt:     time.Now().UTC(),
		Status:        "active",
		Description:   desc,
		RunsCount:     0,
		ModifiedFiles: []string{},
	}

	if err := m.saveMeta(name, info); err != nil {
		return nil, err
	}

	return info, nil
}

// List enumerates all active shadow branches with live modification stats.
func (m *Manager) List() ([]*BranchInfo, error) {
	entries, err := os.ReadDir(m.ShadowDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*BranchInfo{}, nil
		}
		return nil, err
	}

	var results []*BranchInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := m.Get(name)
		if err == nil && info != nil {
			results = append(results, info)
		}
	}

	return results, nil
}

// Get returns the metadata and modified file list for a specific shadow branch.
func (m *Manager) Get(name string) (*BranchInfo, error) {
	targetPath := filepath.Join(m.ShadowDir, name)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, ErrBranchNotFound
	}

	info, err := m.loadMeta(name)
	if err != nil {
		// Fallback reconstructed info
		info = &BranchInfo{
			Name:       name,
			GitBranch:  fmt.Sprintf("msh-shadow-%s", name),
			Path:       targetPath,
			CreatedAt:  time.Now().UTC(),
			Status:     "active",
			BaseRef:    "HEAD",
		}
	}

	// Check modified files inside shadow worktree
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = targetPath
	if out, err := statusCmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var mods []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if len(l) > 3 {
				mods = append(mods, strings.TrimSpace(l[3:]))
			}
		}
		info.ModifiedFiles = mods
	}

	return info, nil
}

// Run executes an arbitrary shell command strictly inside the isolated shadow worktree.
func (m *Manager) Run(name, command string, timeout time.Duration, maxLines int) (*protocol.ExecResponse, error) {
	info, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	session, err := execution.NewSession(info.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create shadow execution session: %w", err)
	}

	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	if maxLines <= 0 {
		maxLines = 500
	}

	req := protocol.ExecRequest{
		Command:        command,
		Cwd:            info.Path,
		Timeout:        timeout,
		MaxOutputLines: maxLines,
		DetectFiles:    true,
		UsePty:         true,
	}

	executor := execution.NewExecutor(session)
	resp := executor.Execute(req)

	// Increment run counter and persist
	info.RunsCount++
	_ = m.saveMeta(name, info)

	return &resp, nil
}

// Diff generates a unified git diff of all modifications made within the shadow branch.
func (m *Manager) Diff(name string) (string, error) {
	info, err := m.Get(name)
	if err != nil {
		return "", err
	}

	// git diff HEAD
	diffCmd := exec.Command("git", "diff", "HEAD")
	diffCmd.Dir = info.Path
	out, err := diffCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to generate diff: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	res := string(out)

	// Also check untracked files
	untrackedCmd := exec.Command("git", "status", "--porcelain")
	untrackedCmd.Dir = info.Path
	if statusOut, err := untrackedCmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "?? ") {
				untrackedFile := strings.TrimSpace(l[3:])
				res += fmt.Sprintf("\n--- /dev/null\n+++ b/%s\n@@ new untracked file @@\n", untrackedFile)
			}
		}
	}

	return res, nil
}

// Merge integrates verified speculative changes back into the main working tree and cleans up.
func (m *Manager) Merge(name, commitMsg string) error {
	info, err := m.Get(name)
	if err != nil {
		return err
	}

	if commitMsg == "" {
		commitMsg = fmt.Sprintf("msh: merge speculative branch '%s'", name)
	}

	// In shadow worktree: add and commit any pending dirty working tree changes
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = info.Path
	_ = addCmd.Run()

	commitCmd := exec.Command("git", "commit", "-m", commitMsg, "--allow-empty")
	commitCmd.Dir = info.Path
	_ = commitCmd.Run()

	// In main working tree: merge the shadow branch
	mergeCmd := exec.Command("git", "merge", "--no-ff", info.GitBranch, "-m", commitMsg)
	mergeCmd.Dir = m.WorkspaceDir
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git merge failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// Clean up worktree and shadow git branch
	return m.cleanupWorktree(name, info.GitBranch)
}

// Abort cleanly discards an exploratory hypothesis without leaving dirty uncommitted state.
// Returns a SemanticPrunePayload indicating how many turns to prune from LLM context.
func (m *Manager) Abort(name string) (*PrunePayload, error) {
	info, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	cleanedFiles := append([]string(nil), info.ModifiedFiles...)
	turnsToPrune := info.RunsCount
	if turnsToPrune <= 0 {
		turnsToPrune = 1
	}

	err = m.cleanupWorktree(name, info.GitBranch)
	if err != nil {
		return nil, err
	}

	payload := &PrunePayload{
		Branch:            name,
		Status:            "aborted",
		AbortedAt:         time.Now().UTC(),
		ContextPruneTurns: turnsToPrune,
		Summary:           fmt.Sprintf("Speculative hypothesis '%s' safely aborted. Main workspace untouched.", name),
		FilesCleaned:      cleanedFiles,
	}

	return payload, nil
}

func (m *Manager) cleanupWorktree(name, gitBranch string) error {
	targetPath := filepath.Join(m.ShadowDir, name)

	// Remove worktree
	removeCmd := exec.Command("git", "worktree", "remove", "--force", targetPath)
	removeCmd.Dir = m.WorkspaceDir
	_ = removeCmd.Run()

	// Prune worktree metadata
	_ = exec.Command("git", "worktree", "prune").Run()

	// Delete git branch
	if gitBranch != "" {
		delCmd := exec.Command("git", "branch", "-D", gitBranch)
		delCmd.Dir = m.WorkspaceDir
		_ = delCmd.Run()
	}

	// Remove directory if lingering
	_ = os.RemoveAll(targetPath)
	return nil
}

func (m *Manager) metaPath(name string) string {
	return filepath.Join(m.ShadowDir, name, ".msh-meta.json")
}

func (m *Manager) saveMeta(name string, info *BranchInfo) error {
	path := m.metaPath(name)
	bytes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

func (m *Manager) loadMeta(name string) (*BranchInfo, error) {
	path := m.metaPath(name)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info BranchInfo
	if err := json.Unmarshal(bytes, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
