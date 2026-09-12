
package protocol

import (
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	t1 := GenerateToken("msh-")
	t2 := GenerateToken("msh-")

	if !strings.HasPrefix(t1, "msh-") {
		t.Fatalf("expected prefix 'msh-', got %s", t1)
	}

	// Should be 4 + 32 = 36 characters
	if len(t1) != 36 {
		t.Fatalf("expected token length 36, got %d (%s)", len(t1), t1)
	}

	if t1 == t2 {
		t.Fatalf("expected unique tokens, got collision: %s == %s", t1, t2)
	}
}

func TestSecureCompare(t *testing.T) {
	if !SecureCompare("secret-token-123", "secret-token-123") {
		t.Fatal("expected identical tokens to match")
	}

	if SecureCompare("secret-token-123", "secret-token-456") {
		t.Fatal("expected different tokens not to match")
	}

	if SecureCompare("secret-token-123", "secret-token-12") {
		t.Fatal("expected different length tokens not to match")
	}

	if SecureCompare("", "token") || SecureCompare("token", "") {
		t.Fatal("expected empty comparison to fail")
	}

	if !SecureCompare("", "") {
		t.Fatal("expected empty strings to match")
	}
}
