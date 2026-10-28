package execution

import (
	"regexp"
	"strings"
)

// readOnlyCommandPatterns matches shell commands that only inspect system or repository state
// without modifying filesystem files. For these commands, spinning up filesystem watchers
// and generating diff trees is unnecessary overhead.
var readOnlyCommandPatterns = []*regexp.Regexp{
	// System / Identity info
	regexp.MustCompile(`^(?:whoami|id|pwd|hostname|uname(?:\s+-[a-zA-Z]+)?|uptime|date)$`),

	// Network inspection
	regexp.MustCompile(`^(?:netstat|ss|ip\s+addr|ifconfig|arp)(?:\s+.*)?$`),

	// Binary location
	regexp.MustCompile(`^(?:which|where|type|command\s+-v)\s+.*$`),

	// Environment & version checks
	regexp.MustCompile(`^(?:env|printenv)$`),
	regexp.MustCompile(`^(?:node|npm|pnpm|yarn|bun|python|python3|go|cargo|rustc|git|docker)\s+(?:-v|--version|-V|version)$`),

	// Process inspection
	regexp.MustCompile(`^(?:ps(?:\s+.*)?|top|tasklist(?:\s+.*)?)$`),

	// Git read-only inspections
	regexp.MustCompile(`^git\s+(?:status|log|diff|branch|show|tag|rev-parse|describe|remote(?:\s+-v)?)(?:\s+.*)?$`),

	// Basic file viewing
	regexp.MustCompile(`^(?:ls|dir)(?:\s+.*)?$`),
	regexp.MustCompile(`^(?:cat|head|tail)\s+.*$`),
}

// IsReadOnlyInspectionCommand returns true if the command is a read-only query
// that does not alter files, allowing msh to take a fast execution path and skip
// filesystem snapshotting.
func IsReadOnlyInspectionCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	// If the command contains output redirections or pipes to modifying tools, it's not read-only
	if strings.Contains(trimmed, ">") || strings.Contains(trimmed, ">>") {
		return false
	}

	for _, pattern := range readOnlyCommandPatterns {
		if pattern.MatchString(trimmed) {
			return true
		}
	}
	return false
}
