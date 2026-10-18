package sanitize

import (
	"testing"
)

func TestRedactEnvValues(t *testing.T) {
	secrets := map[string]string{
		"API_KEY":  "supersecretvalue123",
		"DB_PASS":  "dbpass456",
		"TOO_SMALL": "ab", // ignored: too short
		"WITH_SPACE": "two words", // ignored: contains whitespace
		"UNUSED":   "neverappears",
	}

	input := `connecting with supersecretvalue123 and dbpass456 done`

	out, redacted := RedactEnvValues(input, secrets)

	if !contains(redacted, "API_KEY") || !contains(redacted, "DB_PASS") {
		t.Fatalf("expected API_KEY and DB_PASS to be redacted, got: %v", redacted)
	}
	if contains(redacted, "TOO_SMALL") || contains(redacted, "WITH_SPACE") || contains(redacted, "UNUSED") {
		t.Fatalf("guarded secrets should never be redacted, got: %v", redacted)
	}
	if out != `connecting with [REDACTED:API_KEY] and [REDACTED:DB_PASS] done` {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRedactEnvValuesNoSecrets(t *testing.T) {
	out, redacted := RedactEnvValues("plain text output", nil)
	if out != "plain text output" {
		t.Fatalf("output changed without secrets: %q", out)
	}
	if len(redacted) != 0 {
		t.Fatalf("expected no redactions, got: %v", redacted)
	}
}

func TestRedactKnownPatterns(t *testing.T) {
	input := "key is AKIAIOSFODNN7EXAMPLE and token ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	out, kinds := RedactKnownPatterns(input)

	if !contains(kinds, "aws") || !contains(kinds, "github") {
		t.Fatalf("expected aws and github to be redacted, got: %v", kinds)
	}
	if out != "key is [REDACTED:aws] and token [REDACTED:github]" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRedactKnownPatternsPrivateKey(t *testing.T) {
	input := "BEGIN -----BEGIN RSA PRIVATE KEY-----\nMIIEpA\n-----END RSA PRIVATE KEY----- END"
	out, kinds := RedactKnownPatterns(input)
	if !contains(kinds, "private_key") {
		t.Fatalf("expected private_key to be redacted, got: %v", kinds)
	}
	if out != "BEGIN [REDACTED:private_key] END" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRedactKnownPatternsJWT(t *testing.T) {
	input := "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	out, kinds := RedactKnownPatterns(input)
	if !contains(kinds, "jwt") {
		t.Fatalf("expected jwt to be redacted, got: %v", kinds)
	}
	if out != "token=[REDACTED:jwt]" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRedactSecretsCombined(t *testing.T) {
	secrets := map[string]string{"GITHUB_TOKEN": "ghp_abcdefghijklmnopqrstuvwxyz1234567890"}
	input := "value: ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	out, redacted := RedactSecrets(input, secrets)

	// github pattern should win; the exact-value pass should find nothing left to do
	if !contains(redacted, "github") {
		t.Fatalf("expected github pattern redaction, got: %v", redacted)
	}
	if out != "value: [REDACTED:github]" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRedactSecretsShortValueNotMasked(t *testing.T) {
	input := "hello abc"
	out, redacted := RedactSecrets(input, map[string]string{"SHORT": "abc"})
	if out != "hello abc" {
		t.Fatalf("short value must not be masked: %q", out)
	}
	if len(redacted) != 0 {
		t.Fatalf("expected no redactions, got: %v", redacted)
	}
}

func TestRedactSecretsWhitespaceValueNotMasked(t *testing.T) {
	input := "hello two words"
	out, _ := RedactSecrets(input, map[string]string{"SPACY": "two words"})
	if out != "hello two words" {
		t.Fatalf("whitespace value must not be masked: %q", out)
	}
}

func contains(list []string, item string) bool {
	for _, e := range list {
		if e == item {
			return true
		}
	}
	return false
}