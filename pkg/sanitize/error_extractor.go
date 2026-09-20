package sanitize

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/msh-protocol/msh/pkg/protocol"
)

var (
	// TypeScript: src/auth.ts(42,15): error TS2304: Cannot find name 'foo'.
	reTypeScriptParen = regexp.MustCompile(`(?m)^([^\s:(]+)\((\d+),(\d+)\):\s*error\s+(TS\d+):\s*(.+)$`)
	// TypeScript: src/index.ts:12:5 - error TS2322: Type 'string' is not assignable to type 'number'.
	reTypeScriptColon = regexp.MustCompile(`(?m)^([^\s:]+):(\d+):(\d+)\s*-\s*error\s+(TS\d+):\s*(.+)$`)

	// JavaScript / Node V8 Exception:
	// TypeError: Cannot read properties of undefined (reading 'split')
	//     at Object.<anonymous> (/workspace/src/auth.ts:42:15)
	reNodeException = regexp.MustCompile(`(?m)^(?:UnhandledPromiseRejection:\s*)?([A-Z][a-zA-Z]*Error):\s*(.+)\n(?:\s+at\s+.*?\((?:file:\/\/)?([^:)]+):(\d+):(\d+)\)|\s+at\s+(?:file:\/\/)?([^:)]+):(\d+):(\d+))`)

	// Python Traceback:
	// File "app.py", line 42, in <module>
	//   result = func()
	// ValueError: invalid literal for int()
	rePythonFileTrace = regexp.MustCompile(`(?m)File "([^"]+)", line (\d+)(?:, in .*)?\n(?:\s+(.+)\n)?\s*([A-Z][a-zA-Z]*Error|[A-Z][a-zA-Z]*Exception):\s*(.+)`)

	// Pytest failure:
	// FAILED tests/test_api.py::test_auth - AssertionError: expected 200 got 401
	rePytestFailed = regexp.MustCompile(`(?m)^FAILED\s+([^:]+)::(\S+)\s+-\s+([A-Za-z]+Error|[A-Za-z]+Exception):\s*(.+)$`)

	// Go Compiler:
	// ./main.go:42:15: undefined: foo
	reGoCompiler = regexp.MustCompile(`(?m)^([^\s:]+\.go):(\d+):(\d+):\s*(.+)$`)

	// Go Test Failure:
	// --- FAIL: TestAuth (0.01s)
	//     auth_test.go:42: expected token, got nil
	reGoTest = regexp.MustCompile(`(?m)---\s*FAIL:\s*(\w+)\s*\([^)]*\)\n\s+([^\s:]+\.go):(\d+):\s*(.+)$`)

	// Rust Compiler:
	// error[E0425]: cannot find value 'foo' in this scope
	//   --> src/main.rs:42:15
	reRustCompiler = regexp.MustCompile(`(?m)^error(?:\[(E\d+)\])?:\s*(.+)\n\s+-->\s+([^:]+):(\d+):(\d+)`)

	// C / C++ Compiler (GCC / Clang):
	// main.c:42:15: error: 'foo' undeclared
	reGccCompiler = regexp.MustCompile(`(?m)^([^\s:]+\.[ch](?:pp|xx)?):(\d+):(\d+):\s*(?:fatal\s+)?error:\s*(.+)$`)

	// Generic Shell Errors:
	// bash: line 5: foo: command not found
	// sh: 1: python3: not found
	reShellCmdNotFound = regexp.MustCompile(`(?m)(?:bash|sh|zsh)?:\s*(?:line \d+:\s*)?([^\s:]+):\s*(command not found|not found)`)
	rePermissionDenied = regexp.MustCompile(`(?m)([^\s:]+):\s*(Permission denied)`)
)

// ExtractRootCause analyzes combined stdout and stderr to identify the primary
// language, file, line number, and error message causing a command failure.
func ExtractRootCause(stdout, stderr string) *protocol.ErrorRootCause {
	combined := stderr
	if combined == "" {
		combined = stdout
	} else if stdout != "" {
		combined = stdout + "\n" + stderr
	}

	// 1. Check TypeScript paren format
	if m := reTypeScriptParen.FindStringSubmatch(combined); len(m) > 5 {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return &protocol.ErrorRootCause{
			Type:    m[4], // TSxxxx
			Message: strings.TrimSpace(m[5]),
			File:    m[1],
			Line:    line,
			Column:  col,
		}
	}

	// 2. Check TypeScript colon format
	if m := reTypeScriptColon.FindStringSubmatch(combined); len(m) > 5 {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return &protocol.ErrorRootCause{
			Type:    m[4], // TSxxxx
			Message: strings.TrimSpace(m[5]),
			File:    m[1],
			Line:    line,
			Column:  col,
		}
	}

	// 3. Check Python Traceback
	if m := rePythonFileTrace.FindStringSubmatch(combined); len(m) > 5 {
		line, _ := strconv.Atoi(m[2])
		return &protocol.ErrorRootCause{
			Type:    m[4],
			Message: strings.TrimSpace(m[5]),
			File:    m[1],
			Line:    line,
			Snippet: strings.TrimSpace(m[3]),
		}
	}

	// 4. Check Pytest failure
	if m := rePytestFailed.FindStringSubmatch(combined); len(m) > 4 {
		return &protocol.ErrorRootCause{
			Type:    m[3],
			Message: strings.TrimSpace(m[4]),
			File:    m[1],
			Snippet: m[2],
		}
	}

	// 5. Check Node / JavaScript Exception
	if m := reNodeException.FindStringSubmatch(combined); len(m) > 2 {
		file := m[3]
		lineStr := m[4]
		colStr := m[5]
		if file == "" {
			file = m[6]
			lineStr = m[7]
			colStr = m[8]
		}
		line, _ := strconv.Atoi(lineStr)
		col, _ := strconv.Atoi(colStr)
		return &protocol.ErrorRootCause{
			Type:    m[1],
			Message: strings.TrimSpace(m[2]),
			File:    file,
			Line:    line,
			Column:  col,
		}
	}

	// 6. Check Go Compiler
	if m := reGoCompiler.FindStringSubmatch(combined); len(m) > 4 {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return &protocol.ErrorRootCause{
			Type:    "CompilationError",
			Message: strings.TrimSpace(m[4]),
			File:    m[1],
			Line:    line,
			Column:  col,
		}
	}

	// 7. Check Go Test
	if m := reGoTest.FindStringSubmatch(combined); len(m) > 4 {
		line, _ := strconv.Atoi(m[3])
		return &protocol.ErrorRootCause{
			Type:    "TestFailure",
			Message: strings.TrimSpace(m[4]),
			File:    m[2],
			Line:    line,
			Snippet: m[1],
		}
	}

	// 8. Check Rust Compiler
	if m := reRustCompiler.FindStringSubmatch(combined); len(m) > 5 {
		errType := m[1]
		if errType == "" {
			errType = "RustError"
		}
		line, _ := strconv.Atoi(m[4])
		col, _ := strconv.Atoi(m[5])
		return &protocol.ErrorRootCause{
			Type:    errType,
			Message: strings.TrimSpace(m[2]),
			File:    m[3],
			Line:    line,
			Column:  col,
		}
	}

	// 9. Check GCC / Clang
	if m := reGccCompiler.FindStringSubmatch(combined); len(m) > 4 {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		return &protocol.ErrorRootCause{
			Type:    "CompilerError",
			Message: strings.TrimSpace(m[4]),
			File:    m[1],
			Line:    line,
			Column:  col,
		}
	}

	// 10. Check Shell Command Not Found
	if m := reShellCmdNotFound.FindStringSubmatch(combined); len(m) > 2 {
		return &protocol.ErrorRootCause{
			Type:    "CommandNotFound",
			Message: m[1] + ": " + m[2],
		}
	}

	// 11. Check Permission Denied
	if m := rePermissionDenied.FindStringSubmatch(combined); len(m) > 2 {
		return &protocol.ErrorRootCause{
			Type:    "PermissionDenied",
			Message: m[1] + ": " + m[2],
		}
	}

	return nil
}
