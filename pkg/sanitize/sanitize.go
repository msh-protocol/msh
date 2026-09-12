// Package sanitize provides output cleaning utilities for the msh runtime.
//
// AI agents consume stdout/stderr text as context tokens. Raw terminal output
// contains ANSI escape codes, interactive prompts, and massive log dumps that
// waste tokens and confuse models. This package strips, truncates, and detects
// these patterns to produce clean, machine-friendly output.
package sanitize

import (
	"fmt"
	"regexp"
	"strings"
)

// ansiPattern matches all ANSI escape sequences including:
//   - CSI sequences: \033[ ... (letter)  — colors, cursor movement
//   - OSC sequences: \033] ... \033\\     — terminal titles, hyperlinks
//   - Simple escapes: \033 (letter)       — cursor save/restore
var ansiPattern = regexp.MustCompile(`(\x1b\[[0-9;?]*[a-zA-Z])|(\x1b\][^\x07\x1b]*(?:\x07|\x1b\\))|(\x1b[()][0-9A-B])|(\x1b[a-zA-Z])`)

// carriageReturnLine matches lines that use \r for progress bar overwrites.
// These create visual noise that wastes agent context.
var carriageReturnLine = regexp.MustCompile(`\r[^\n]`)

// StripANSI removes all ANSI escape codes from the input byte slice.
// This converts colorized, cursor-manipulated terminal output into
// plain text that AI agents can consume without wasting tokens.
//
// Example:
//
//	Input:  "\033[0;31mError\033[0m: file not found"
//	Output: "Error: file not found"
func StripANSI(input []byte) []byte {
	cleaned := ansiPattern.ReplaceAll(input, nil)
	// Strip carriage return overwrites (progress bars, spinners)
	cleaned = carriageReturnLine.ReplaceAll(cleaned, nil)
	return cleaned
}

// StripANSIString is a convenience wrapper around StripANSI for strings.
func StripANSIString(input string) string {
	return string(StripANSI([]byte(input)))
}

// TruncateOutput limits output to maxLines lines. If the output exceeds
// the limit, it keeps the first half and last half of lines with a
// truncation marker in between, preserving both the initial context
// and the final error/result.
//
// This is critical for token economy — a 10,000-line error dump would
// cost the agent massive API fees. Truncation keeps only what matters.
//
// Returns the truncated string and a boolean indicating whether
// truncation occurred.
func TruncateOutput(input string, maxLines int) (string, bool) {
	if maxLines <= 0 {
		maxLines = 200
	}

	lines := strings.Split(input, "\n")

	// Remove trailing empty line from split
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) <= maxLines {
		return input, false
	}

	// Keep first half and last half
	headCount := maxLines / 2
	tailCount := maxLines - headCount
	omitted := len(lines) - maxLines

	head := lines[:headCount]
	tail := lines[len(lines)-tailCount:]

	marker := fmt.Sprintf("\n... [msh: truncated %d lines to save tokens] ...\n", omitted)

	result := strings.Join(head, "\n") + marker + strings.Join(tail, "\n")
	return result, true
}

// promptPatterns contains regex patterns for common interactive prompts
// that block agent execution loops.
var promptPatterns = []*regexp.Regexp{
	// Yes/No prompts
	regexp.MustCompile(`(?i)\[y/n\]`),
	regexp.MustCompile(`(?i)\[yes/no\]`),
	regexp.MustCompile(`(?i)\(y/n\)`),
	regexp.MustCompile(`(?i)\(yes\)`),
	regexp.MustCompile(`(?i)\?\s*\([y/n]\)`),
	regexp.MustCompile(`(?i)continue\?\s*\[`),
	regexp.MustCompile(`(?i)proceed\?\s*\[`),
	regexp.MustCompile(`(?i)overwrite\?\s*\(`),

	// Password/auth prompts
	regexp.MustCompile(`(?i)password\s*:`),
	regexp.MustCompile(`(?i)passphrase.*:`),
	regexp.MustCompile(`(?i)enter\s+password`),
	regexp.MustCompile(`(?i)sudo.*password`),

	// Confirmation prompts
	regexp.MustCompile(`(?i)are you sure`),
	regexp.MustCompile(`(?i)is this ok`),
	regexp.MustCompile(`(?i)press enter to continue`),
	regexp.MustCompile(`(?i)press any key`),
	regexp.MustCompile(`(?i)hit enter`),

	// Package manager prompts
	regexp.MustCompile(`(?i)do you want to install`),
	regexp.MustCompile(`(?i)need to install`),
	regexp.MustCompile(`(?i)ok to proceed`),
}

// DetectPrompt checks if a line of output contains an interactive prompt
// that would block execution. Returns the matched prompt text and true
// if a prompt is detected.
//
// This prevents the classic agent hang: an agent runs a command that asks
// "Do you want to continue? [y/N]" and the execution loop freezes forever.
func DetectPrompt(line string) (string, bool) {
	for _, pattern := range promptPatterns {
		if loc := pattern.FindStringIndex(line); loc != nil {
			// Return the surrounding context of the prompt
			start := loc[0]
			if start > 20 {
				start = loc[0] - 20
			} else {
				start = 0
			}
			end := loc[1]
			if end+20 < len(line) {
				end = loc[1] + 20
			} else {
				end = len(line)
			}
			return strings.TrimSpace(line[start:end]), true
		}
	}
	return "", false
}

// CleanOutput applies the full sanitization pipeline to raw command output:
// 1. Strip ANSI escape codes
// 2. Truncate to maxLines
// Returns the cleaned output and whether truncation occurred.
func CleanOutput(raw string, maxLines int) (string, bool) {
	stripped := StripANSIString(raw)
	return TruncateOutput(stripped, maxLines)
}
