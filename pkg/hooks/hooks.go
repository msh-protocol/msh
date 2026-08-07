package hooks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/msh-protocol/msh/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// HookDef defines a single hook execution.
type HookDef struct {
	Name        string `yaml:"name"`
	Command     string `yaml:"command"`
	BlockOnFail bool   `yaml:"block_on_fail"`
}

// Config represents the .msh/hooks.yaml configuration file.
type Config struct {
	Hooks struct {
		PreExec  []HookDef `yaml:"pre_exec"`
		PostExec []HookDef `yaml:"post_exec"`
	} `yaml:"hooks"`
}

// LoadConfig attempts to load .msh/hooks.yaml from the given workspace root.
func LoadConfig(workspaceRoot string) (*Config, error) {
	configPath := filepath.Join(workspaceRoot, ".msh", "hooks.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil // No hooks file, return empty config
		}
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	return &config, nil
}

// RunPreHooks executes all pre_exec hooks defined in the configuration.
// If any hook fails and BlockOnFail is true, it returns the HookResult and an error.
func RunPreHooks(ctx context.Context, cfg *Config, originalCmd string, cwd string) ([]protocol.HookResult, error) {
	var results []protocol.HookResult

	for _, hook := range cfg.Hooks.PreExec {
		res := executeHook(ctx, hook, "pre_exec", originalCmd, cwd)
		results = append(results, res)

		if res.ExitCode != 0 && hook.BlockOnFail {
			return results, fmt.Errorf("pre_exec hook %q blocked execution (exit code %d)", hook.Name, res.ExitCode)
		}
	}

	return results, nil
}

// RunPostHooks executes all post_exec hooks defined in the configuration.
// Since post_exec hooks shouldn't block the main response, errors are logged but not returned.
func RunPostHooks(ctx context.Context, cfg *Config, originalCmd string, cwd string) []protocol.HookResult {
	var results []protocol.HookResult

	for _, hook := range cfg.Hooks.PostExec {
		res := executeHook(ctx, hook, "post_exec", originalCmd, cwd)
		results = append(results, res)
	}

	return results
}

func executeHook(ctx context.Context, hook HookDef, hookType string, originalCmd string, cwd string) protocol.HookResult {
	// Inject the original command into the hook command
	cmdStr := strings.ReplaceAll(hook.Command, "{{command}}", originalCmd)

	res := protocol.HookResult{
		Name:    hook.Name,
		Type:    hookType,
		Command: cmdStr,
	}

	// For safety, add a 1-minute timeout to hooks to prevent them from hanging indefinitely
	hookCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(hookCtx, "cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(hookCtx, "sh", "-c", cmdStr)
	}
	cmd.Dir = cwd

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	
	res.Stdout = stdoutBuf.String()
	res.Stderr = stderrBuf.String()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitError.ExitCode()
		} else {
			res.ExitCode = -1 // System error (e.g. timeout)
			res.Stderr += fmt.Sprintf("\n[msh hook error]: %v", err)
		}
	} else {
		res.ExitCode = 0
	}

	return res
}
