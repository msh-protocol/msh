package sanitize

import (
	"testing"
)

func TestExtractRootCause_TypeScript(t *testing.T) {
	output := `
src/auth.ts(42,15): error TS2304: Cannot find name 'jwtSecret'.
src/auth.ts(45,3): error TS2322: Type 'string' is not assignable to type 'number'.
`
	cause := ExtractRootCause(output, "")
	if cause == nil {
		t.Fatal("expected root cause, got nil")
	}
	if cause.Type != "TS2304" {
		t.Errorf("expected TS2304, got %s", cause.Type)
	}
	if cause.File != "src/auth.ts" {
		t.Errorf("expected src/auth.ts, got %s", cause.File)
	}
	if cause.Line != 42 || cause.Column != 15 {
		t.Errorf("expected line 42 col 15, got line %d col %d", cause.Line, cause.Column)
	}
}

func TestExtractRootCause_Python(t *testing.T) {
	output := `
Traceback (most recent call last):
  File "server.py", line 88, in <module>
    app.start()
  File "app/core.py", line 15, in start
    x = 1 / 0
ZeroDivisionError: division by zero
`
	cause := ExtractRootCause(output, "")
	if cause == nil {
		t.Fatal("expected root cause, got nil")
	}
	if cause.Type != "ZeroDivisionError" {
		t.Errorf("expected ZeroDivisionError, got %s", cause.Type)
	}
	if cause.File != "app/core.py" {
		t.Errorf("expected app/core.py, got %s", cause.File)
	}
	if cause.Line != 15 {
		t.Errorf("expected line 15, got %d", cause.Line)
	}
	if cause.Message != "division by zero" {
		t.Errorf("expected 'division by zero', got '%s'", cause.Message)
	}
}

func TestExtractRootCause_GoCompiler(t *testing.T) {
	output := `
# github.com/example/app
./main.go:27:12: undefined: CalculateMetric
`
	cause := ExtractRootCause("", output)
	if cause == nil {
		t.Fatal("expected root cause, got nil")
	}
	if cause.File != "./main.go" {
		t.Errorf("expected ./main.go, got %s", cause.File)
	}
	if cause.Line != 27 || cause.Column != 12 {
		t.Errorf("expected line 27 col 12, got line %d col %d", cause.Line, cause.Column)
	}
	if cause.Message != "undefined: CalculateMetric" {
		t.Errorf("expected 'undefined: CalculateMetric', got '%s'", cause.Message)
	}
}

func TestExtractRootCause_RustCompiler(t *testing.T) {
	output := `
error[E0425]: cannot find value 'token' in this scope
  --> src/auth.rs:34:9
   |
34 |         token.verify()
   |         ^^^^^ not found in this scope
`
	cause := ExtractRootCause(output, "")
	if cause == nil {
		t.Fatal("expected root cause, got nil")
	}
	if cause.Type != "E0425" {
		t.Errorf("expected E0425, got %s", cause.Type)
	}
	if cause.File != "src/auth.rs" {
		t.Errorf("expected src/auth.rs, got %s", cause.File)
	}
	if cause.Line != 34 || cause.Column != 9 {
		t.Errorf("expected line 34 col 9, got line %d col %d", cause.Line, cause.Column)
	}
}

func TestExtractRootCause_CommandNotFound(t *testing.T) {
	output := "bash: line 1: nonexistingcmd: command not found"
	cause := ExtractRootCause("", output)
	if cause == nil {
		t.Fatal("expected root cause, got nil")
	}
	if cause.Type != "CommandNotFound" {
		t.Errorf("expected CommandNotFound, got %s", cause.Type)
	}
}
