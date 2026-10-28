package execution

import (
	"testing"
)

func TestIsPersistentServerCommand(t *testing.T) {
	cases := []struct {
		cmd      string
		expected bool
	}{
		{"npm run dev", true},
		{"npm run start", true},
		{"npm start", true},
		{"pnpm dev", true},
		{"yarn dev", true},
		{"bun run dev", true},
		{"next dev", true},
		{"next start", true},
		{"vite", true},
		{"vite dev", true},
		{"nuxt dev", true},
		{"astro dev", true},
		{"python -m http.server", true},
		{"python3 -m http.server 8000", true},
		{"uvicorn main:app --reload", true},
		{"gunicorn app:app", true},
		{"flask run", true},
		{"docker compose up", true},
		{"docker-compose up -d", true},
		{"cargo watch -x run", true},

		// Standard commands (should NOT be detected as persistent servers)
		{"git status", false},
		{"npm test", false},
		{"npm run build", false},
		{"pytest -v", false},
		{"go test ./...", false},
		{"ls -la", false},
		{"cat package.json", false},
	}

	for _, c := range cases {
		got := IsPersistentServerCommand(c.cmd)
		if got != c.expected {
			t.Errorf("IsPersistentServerCommand(%q) = %v; want %v", c.cmd, got, c.expected)
		}
	}
}

func TestIsReadOnlyInspectionCommand(t *testing.T) {
	cases := []struct {
		cmd      string
		expected bool
	}{
		{"whoami", true},
		{"id", true},
		{"pwd", true},
		{"netstat -an", true},
		{"ss -tulpn", true},
		{"which node", true},
		{"where git", true},
		{"hostname", true},
		{"uname -a", true},
		{"node -v", true},
		{"python --version", true},
		{"git status", true},
		{"git log -n 5", true},
		{"git diff", true},
		{"git branch", true},
		{"ls -la", true},
		{"cat README.md", true},

		// Modifying commands (MUST NOT be read-only)
		{"touch newfile.txt", false},
		{"rm -rf /tmp/foo", false},
		{"npm install", false},
		{"git commit -m 'test'", false},
		{"echo hello > file.txt", false},
		{"echo hello >> file.txt", false},
		{"go build ./...", false},
	}

	for _, c := range cases {
		got := IsReadOnlyInspectionCommand(c.cmd)
		if got != c.expected {
			t.Errorf("IsReadOnlyInspectionCommand(%q) = %v; want %v", c.cmd, got, c.expected)
		}
	}
}
