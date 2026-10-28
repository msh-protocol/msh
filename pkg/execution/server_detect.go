package execution

import (
	"regexp"
	"strings"
	"time"
)

// UnboundedServerTimeout is the timeout duration applied to persistent dev servers
// and long-running services when the caller has not specified an explicit timeout.
const UnboundedServerTimeout = 24 * time.Hour

// persistentCommandPatterns matches shell commands known to run persistent
// development servers, watchers, or network listeners.
var persistentCommandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:npm|pnpm|yarn|bun)\s+(?:run\s+)?(?:dev|start|serve|watch)\b`),
	regexp.MustCompile(`(?i)\b(?:next|nuxt|astro|remix|gatsby)\s+(?:dev|start)\b`),
	regexp.MustCompile(`(?i)\bvite(?:\s+(?:dev|preview))?\b`),
	regexp.MustCompile(`(?i)\b(?:webpack-dev-server|webpack\s+serve)\b`),
	regexp.MustCompile(`(?i)\bnodemon\b`),

	// Python web servers
	regexp.MustCompile(`(?i)\bpython[0-9.]*\s+-m\s+http\.server\b`),
	regexp.MustCompile(`(?i)\b(?:uvicorn|gunicorn|waitress-serve)\b`),
	regexp.MustCompile(`(?i)\bflask\s+run\b`),
	regexp.MustCompile(`(?i)\bdjango-admin\s+runserver\b`),
	regexp.MustCompile(`(?i)\bpython[0-9.]*\s+manage\.py\s+runserver\b`),

	// Ruby / Rails
	regexp.MustCompile(`(?i)\brails\s+(?:server|s)\b`),

	// Rust / Go / Other watchers
	regexp.MustCompile(`(?i)\bcargo\s+watch\b`),
	regexp.MustCompile(`(?i)\bair\b`), // Go hot reload

	// Containers & services
	regexp.MustCompile(`(?i)\bdocker(?:-compose|\s+compose)\s+up\b`),
}

// IsPersistentServerCommand checks if the command string matches a known persistent
// dev server or listener that is intended to run indefinitely.
func IsPersistentServerCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	for _, pattern := range persistentCommandPatterns {
		if pattern.MatchString(trimmed) {
			return true
		}
	}
	return false
}
