package sanitize

import (
	"regexp"
	"strings"
)

// Secret redaction protects AI agents from leaking API keys, tokens, and
// other secrets directly into model context windows.
//
// Two redaction layers are applied:
//  1. EnvValue redaction — replaces the exact value of known secrets
//     (API keys, .env values, session env vars) wherever it appears.
//  2. Pattern redaction — masks well-known secret formats (AWS access keys,
//     GitHub tokens, Slack tokens, JWT, private key blocks) by shape.

// envValueGuard excludes values that are too short or contain whitespace,
// which would otherwise cause over-aggressive replacement of ordinary text.
func envValueGuard(value string) bool {
	return len(value) >= 4 && !strings.ContainsAny(value, " \t\r\n")
}

// redactSecretPatterns masks well-known secret formats by shape.
var redactSecretPatterns = []struct {
	kind    string
	pattern *regexp.Regexp
}{
	{"aws", regexp.MustCompile(`(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`)},
	{"github", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"github_pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	{"slack", regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{10,48}`)},
	{"openai", regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`)},
	{"anthropic", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{"stripe", regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24,}`)},
	{"stripe_test", regexp.MustCompile(`sk_test_[0-9a-zA-Z]{24,}`)},
	{"google_api", regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
	{"npm_token", regexp.MustCompile(`npm_[A-Za-z0-9]{36}`)},
	{"private_key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
}

// RedactKnownPatterns masks secrets by their shape (AWS keys, GitHub tokens,
// JWTs, private key blocks, etc.) regardless of the process environment.
// Returns the sanitized output and a deduplicated list of masked secret kinds.
func RedactKnownPatterns(input string) (string, []string) {
	var masked []string
	out := input
	for _, red := range redactSecretPatterns {
		if red.pattern.MatchString(out) {
			out = red.pattern.ReplaceAllString(out, "[REDACTED:"+red.kind+"]")
			masked = append(masked, red.kind)
		}
	}
	return out, masked
}

// RedactEnvValues replaces the exact values of the given secrets wherever
// they appear in the output. Values that are too short or contain whitespace
// are ignored to avoid corrupting ordinary output.
// Returns the sanitized output and the list of secret names that were masked.
func RedactEnvValues(input string, secrets map[string]string) (string, []string) {
	var masked []string
	out := input
	for name, value := range secrets {
		if value == "" || !envValueGuard(value) {
			continue
		}
		if strings.Contains(out, value) {
			out = strings.ReplaceAll(out, value, "[REDACTED:"+name+"]")
			masked = append(masked, name)
		}
	}
	return out, masked
}

// RedactSecrets applies both redaction layers to command output:
//  1. Known-pattern masking (AWS, GitHub, Slack, OpenAI, JWT, keys...)
//  2. Exact-value masking of the provided secrets (env vars, .env values)
//
// Returns the sanitized output and a deduplicated list of what was redacted.
func RedactSecrets(input string, secrets map[string]string) (string, []string) {
	var redacted []string

	out, kinds := RedactKnownPatterns(input)
	redacted = append(redacted, kinds...)

	out, names := RedactEnvValues(out, secrets)
	redacted = append(redacted, names...)

	return out, dedupe(redacted)
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}