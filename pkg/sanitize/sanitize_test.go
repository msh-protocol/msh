package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "strip color codes",
			input: "\033[0;31mError\033[0m: file not found",
			want:  "Error: file not found",
		},
		{
			name:  "strip bold and underline",
			input: "\033[1mBold\033[0m and \033[4mUnderline\033[0m",
			want:  "Bold and Underline",
		},
		{
			name:  "strip 256 color codes",
			input: "\033[38;5;196mRed text\033[0m",
			want:  "Red text",
		},
		{
			name:  "strip cursor movement",
			input: "\033[2J\033[H\033[?25lContent",
			want:  "Content",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "multiple color codes in sequence",
			input: "\033[1m\033[31m\033[42mStyled\033[0m rest",
			want:  "Styled rest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSIString(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLines  int
		wantTrunc bool
		wantLines int // approximate lines in output
	}{
		{
			name:      "short output not truncated",
			input:     "line1\nline2\nline3",
			maxLines:  10,
			wantTrunc: false,
		},
		{
			name:      "exact limit not truncated",
			input:     "line1\nline2\nline3\nline4\nline5",
			maxLines:  5,
			wantTrunc: false,
		},
		{
			name:      "exceeds limit truncated",
			input:     generateLines(100),
			maxLines:  20,
			wantTrunc: true,
		},
		{
			name:      "default max lines used when zero",
			input:     "line1\nline2",
			maxLines:  0,
			wantTrunc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, truncated := TruncateOutput(tt.input, tt.maxLines)
			if truncated != tt.wantTrunc {
				t.Errorf("TruncateOutput() truncated = %v, want %v", truncated, tt.wantTrunc)
			}
			if truncated {
				// Should contain the truncation marker
				if !strings.Contains(result, "[msh: truncated") {
					t.Error("Truncated output should contain truncation marker")
				}
			}
			_ = result
		})
	}
}

func TestTruncateKeepsHeadAndTail(t *testing.T) {
	input := generateLines(100)
	result, truncated := TruncateOutput(input, 20)
	if !truncated {
		t.Fatal("Expected truncation")
	}

	// Should contain first line
	if !strings.Contains(result, "line-0") {
		t.Error("Truncated output should contain the first line")
	}

	// Should contain last line
	if !strings.Contains(result, "line-99") {
		t.Error("Truncated output should contain the last line")
	}

	// Should contain truncation marker with count
	if !strings.Contains(result, "truncated 80 lines") {
		t.Error("Should indicate 80 lines were truncated")
	}
}

func TestDetectPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		detected bool
	}{
		{
			name:     "yes/no prompt",
			input:    "Do you want to continue? [y/N]",
			detected: true,
		},
		{
			name:     "password prompt",
			input:    "Enter password:",
			detected: true,
		},
		{
			name:     "sudo prompt",
			input:    "[sudo] password for user:",
			detected: true,
		},
		{
			name:     "are you sure",
			input:    "Are you sure you want to delete?",
			detected: true,
		},
		{
			name:     "normal output",
			input:    "Build completed successfully.",
			detected: false,
		},
		{
			name:     "npm ok to proceed",
			input:    "Ok to proceed? (y)",
			detected: true,
		},
		{
			name:     "empty string",
			input:    "",
			detected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, found := DetectPrompt(tt.input)
			if found != tt.detected {
				t.Errorf("DetectPrompt(%q) = (_, %v), want (_, %v)", tt.input, found, tt.detected)
			}
			if found && prompt == "" {
				t.Error("Detected prompt should return non-empty prompt text")
			}
		})
	}
}

func TestCleanOutput(t *testing.T) {
	// Combined pipeline test
	input := "\033[31mError:\033[0m something went wrong\n" +
		"Details follow\n" +
		"Stack trace line"

	result, truncated := CleanOutput(input, 100)

	if truncated {
		t.Error("Short output should not be truncated")
	}

	// Should not contain ANSI codes
	if strings.Contains(result, "\033") {
		t.Error("Output should not contain ANSI escape codes")
	}

	// Should still contain the text content
	if !strings.Contains(result, "Error:") {
		t.Error("Output should contain 'Error:'")
	}
}

// generateLines creates a test string with N numbered lines.
func generateLines(n int) string {
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		lines[i] = fmt.Sprintf("line-%d: some content here", i)
	}
	return strings.Join(lines, "\n")
}
