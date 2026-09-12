package execution

import (
	"bytes"
	"io"
	"strings"
	"sync"

	"github.com/msh-protocol/msh/pkg/sanitize"
)

// promptAnswerer hands queued answers to a process stdin whenever an
// interactive prompt is detected in the process output stream. This is what
// turns a "blocked" execution into a completed one: instead of killing the
// process on "[y/N]", msh feeds the agent's answer and lets it continue.
type promptAnswerer struct {
	mu       sync.Mutex
	stdin    io.Writer
	answers  []string
	index    int
	answered int
}

// newPromptAnswerer creates an answerer that writes to stdin.
func newPromptAnswerer(stdin io.Writer, answers []string) *promptAnswerer {
	return &promptAnswerer{
		stdin:   stdin,
		answers: answers,
	}
}

// answeredCount returns how many answers were consumed so far.
func (p *promptAnswerer) answeredCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.answered
}

// respondFor scans the given text for an interactive prompt. If one is found
// and answers remain, the next answer is written to the process stdin.
// Returns true if an answer was fed.
func (p *promptAnswerer) respondFor(text string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.index >= len(p.answers) {
		return false
	}
	if _, ok := sanitize.DetectPrompt(text); !ok {
		return false
	}

	answer := p.answers[p.index]
	p.index++
	p.answered++
	_, _ = io.WriteString(p.stdin, answer+"\n")
	return true
}

// streamScanner relays raw output chunks into an output buffer while feeding
// answers whenever a prompt appears. It scans completed lines plus the
// trailing partial chunk, since prompts often lack a trailing newline.
type streamScanner struct {
	out      *bytes.Buffer
	answerer *promptAnswerer
	tail     string
}

// Write implements io.Writer.
func (s *streamScanner) Write(chunk []byte) (int, error) {
	s.out.Write(chunk)
	s.tail += string(chunk)

	for {
		idx := strings.IndexByte(s.tail, '\n')
		if idx < 0 {
			break
		}
		line := s.tail[:idx]
		s.tail = s.tail[idx+1:]
		s.answerer.respondFor(line)
	}

	// Scan the trailing partial chunk too (e.g. "Do you want to continue? [y/N]: ")
	// and clear it once answered so the same prompt cannot re-fire.
	if len(s.tail) > 0 && s.answerer.respondFor(s.tail) {
		s.tail = ""
	}

	return len(chunk), nil
}