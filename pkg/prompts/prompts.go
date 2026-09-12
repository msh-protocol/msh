// Package prompts implements the .msh/prompts.yaml answer policy file. An
// agent or team can commit reusable prompt answers (e.g. always answer "n" to
// an install confirmation) so runs don't repeat the same --answer flags.
package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Rule maps a detected prompt (matched as a regular expression) to a reusable
// answer that is fed to the process whenever that prompt appears and no
// explicit pre-supplied answer is available.
type Rule struct {
	Match  string `yaml:"match"`
	Answer string `yaml:"answer"`
	re     *regexp.Regexp
}

// Config represents the .msh/prompts.yaml configuration file.
type Config struct {
	Prompts []Rule `yaml:"prompts"`
}

// AnswerFunc resolves an answer for a detected prompt. It is the wire form
// passed into the execution engines.
type AnswerFunc func(prompt string) (answer string, ok bool)

// LoadConfig attempts to load .msh/prompts.yaml from the given workspace root.
// A missing file yields an empty (no-op) config; a malformed file returns an
// error so misconfiguration is surfaced instead of silently ignored.
func LoadConfig(workspaceRoot string) (*Config, error) {
	configPath := filepath.Join(workspaceRoot, ".msh", "prompts.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	for i := range config.Prompts {
		rule := &config.Prompts[i]
		re, err := regexp.Compile(rule.Match)
		if err != nil {
			return nil, fmt.Errorf("invalid prompt match %q in %s: %w", rule.Match, configPath, err)
		}
		rule.re = re
	}

	return &config, nil
}

// AnswerFor returns the answer of the first rule matching the prompt text.
func (c *Config) AnswerFor(prompt string) (string, bool) {
	if c == nil {
		return "", false
	}
	for i := range c.Prompts {
		rule := &c.Prompts[i]
		if rule.re != nil && rule.re.MatchString(prompt) {
			return rule.Answer, true
		}
	}
	return "", false
}

// Resolver exposes an AnswerFunc for a config; it is nil when no rules exist.
func (c *Config) Resolver() AnswerFunc {
	if c == nil || len(c.Prompts) == 0 {
		return nil
	}
	return c.AnswerFor
}