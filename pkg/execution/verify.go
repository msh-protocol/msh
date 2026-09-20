package execution

import (
	"fmt"
	"math"
	"strings"

	"github.com/msh-protocol/msh/pkg/protocol"
)

// VerifyExecution re-executes a command with identical request parameters and evaluates
// whether its execution behavior, exit code, stdout/stderr, and filesystem changes are deterministic.
func VerifyExecution(req protocol.ExecRequest, originalResp protocol.ExecResponse) (*protocol.VerificationResult, error) {
	// 1. Ensure original RunHash is populated
	origHash := originalResp.RunHash
	if origHash == "" {
		origHash = computeRunHash(req.Command, originalResp.Cwd, originalResp.ExitCode, originalResp.Stdout, originalResp.Stderr, originalResp.FilesChanged)
	}

	// 2. Setup isolated verification session
	verifySessionID := "verify-" + originalResp.SessionID
	session, err := NewSession(originalResp.Cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verification session: %w", err)
	}
	session.ID = verifySessionID
	executor := NewExecutor(session)

	// 3. Re-execute command
	replayResp := executor.Execute(req)

	// 4. Evaluate reproducibility
	exitCodeMatch := (replayResp.ExitCode == originalResp.ExitCode)
	outputMatch := (replayResp.Stdout == originalResp.Stdout && replayResp.Stderr == originalResp.Stderr)

	// Compare changed files count and entries
	diffMatch := (len(replayResp.FilesChanged) == len(originalResp.FilesChanged))
	if diffMatch {
		origMap := make(map[string]bool)
		for _, f := range originalResp.FilesChanged {
			origMap[f] = true
		}
		for _, f := range replayResp.FilesChanged {
			if !origMap[f] {
				diffMatch = false
				break
			}
		}
	}

	simScore := calculateLineSimilarity(originalResp.Stdout, replayResp.Stdout)
	if outputMatch {
		simScore = 1.0
	}

	isDeterministic := exitCodeMatch && outputMatch && diffMatch && (replayResp.RunHash == origHash)

	var driftParts []string
	if !exitCodeMatch {
		driftParts = append(driftParts, fmt.Sprintf("Exit code drifted (%d -> %d)", originalResp.ExitCode, replayResp.ExitCode))
	}
	if !outputMatch {
		driftParts = append(driftParts, fmt.Sprintf("Output drifted (similarity: %.1f%%)", simScore*100))
	}
	if !diffMatch {
		driftParts = append(driftParts, fmt.Sprintf("Files changed drifted (%d -> %d files)", len(originalResp.FilesChanged), len(replayResp.FilesChanged)))
	}

	driftSummary := "Bit-for-bit deterministic reproduction"
	if len(driftParts) > 0 {
		driftSummary = strings.Join(driftParts, "; ")
	}

	return &protocol.VerificationResult{
		Command:         req.Command,
		IsDeterministic: isDeterministic,
		ExitCodeMatch:   exitCodeMatch,
		OutputMatch:     outputMatch,
		DiffMatch:       diffMatch,
		SimilarityScore: math.Round(simScore*100) / 100,
		OriginalHash:    origHash,
		ReplayHash:      replayResp.RunHash,
		DriftSummary:    driftSummary,
	}, nil
}

// calculateLineSimilarity computes Jaccard line similarity between two output strings.
func calculateLineSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	lines1 := strings.Split(strings.TrimSpace(s1), "\n")
	lines2 := strings.Split(strings.TrimSpace(s2), "\n")
	if len(lines1) == 0 && len(lines2) == 0 {
		return 1.0
	}

	set1 := make(map[string]int)
	for _, l := range lines1 {
		set1[strings.TrimSpace(l)]++
	}

	intersection := 0
	set2 := make(map[string]int)
	for _, l := range lines2 {
		line := strings.TrimSpace(l)
		set2[line]++
		if set1[line] > 0 {
			intersection++
			set1[line]--
		}
	}

	total := len(lines1) + len(lines2) - intersection
	if total == 0 {
		return 1.0
	}
	return float64(intersection) / float64(total)
}
