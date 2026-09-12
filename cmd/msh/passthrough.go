package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/msh-protocol/msh/pkg/execution"
	"github.com/msh-protocol/msh/pkg/protocol"
)

// PassthroughFlags represents configuration options parsed from the command line
// when running in passthrough mode.
type PassthroughFlags struct {
	Cwd      string
	Timeout  time.Duration
	MaxLines int
	Pty      bool
	NoFiles  bool
	EnvFile  string
	NoRedact bool
	Answers  []string
}

// DefaultPassthroughFlags returns standard production defaults for passthrough runs.
func DefaultPassthroughFlags() PassthroughFlags {
	return PassthroughFlags{
		Timeout:  5 * time.Minute,
		MaxLines: 500,
	}
}

// internalSubcommands is the canonical set of subcommands registered with msh.
var internalSubcommands = map[string]bool{
	"exec":       true,
	"wrap":       true,
	"fleet":      true,
	"daemon":     true,
	"mcp":        true,
	"serve":      true,
	"plugin":     true,
	"version":    true,
	"help":       true,
	"completion": true,
}

// IsInternalSubcommand returns true if the specified name is a recognized msh management command.
func IsInternalSubcommand(name string) bool {
	return internalSubcommands[strings.ToLower(name)]
}

// ParsePassthroughArgs inspects os.Args[1:], extracts any leading msh flags,
// and determines whether the invocation should be routed to passthrough mode.
func ParsePassthroughArgs(args []string) (flags PassthroughFlags, cmdStr string, isPassthrough bool) {
	flags = DefaultPassthroughFlags()
	if len(args) == 0 {
		return flags, "", false
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		// End of options indicator
		if arg == "--" {
			i++
			break
		}

		// Not a flag -> this is the start of the command!
		if !strings.HasPrefix(arg, "-") {
			break
		}

		// Standard help/version flags should pass through to Cobra
		if arg == "-h" || arg == "--help" || arg == "-v" || arg == "--version" {
			return flags, "", false
		}

		// Parse known msh wrapper flags
		if arg == "--cwd" && i+1 < len(args) {
			flags.Cwd = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "--cwd=") {
			flags.Cwd = strings.TrimPrefix(arg, "--cwd=")
			i++
		} else if arg == "--timeout" && i+1 < len(args) {
			if d, err := time.ParseDuration(args[i+1]); err == nil {
				flags.Timeout = d
			}
			i += 2
		} else if strings.HasPrefix(arg, "--timeout=") {
			if d, err := time.ParseDuration(strings.TrimPrefix(arg, "--timeout=")); err == nil {
				flags.Timeout = d
			}
			i++
		} else if (arg == "--max-lines" || arg == "-m") && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				flags.MaxLines = n
			}
			i += 2
		} else if strings.HasPrefix(arg, "--max-lines=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-lines=")); err == nil {
				flags.MaxLines = n
			}
			i++
		} else if arg == "--pty" || arg == "-p" {
			flags.Pty = true
			i++
		} else if arg == "--no-files" {
			flags.NoFiles = true
			i++
		} else if arg == "--no-redact" {
			flags.NoRedact = true
			i++
		} else if arg == "--env-file" && i+1 < len(args) {
			flags.EnvFile = args[i+1]
			i += 2
		} else if strings.HasPrefix(arg, "--env-file=") {
			flags.EnvFile = strings.TrimPrefix(arg, "--env-file=")
			i++
		} else if (arg == "--answer" || arg == "-a") && i+1 < len(args) {
			flags.Answers = append(flags.Answers, args[i+1])
			i += 2
		} else if strings.HasPrefix(arg, "--answer=") {
			flags.Answers = append(flags.Answers, strings.TrimPrefix(arg, "--answer="))
			i++
		} else {
			// Unrecognized leading flag: let Cobra handle it (or return error)
			return flags, "", false
		}
	}

	// If no command arguments remain after parsing flags, not passthrough
	if i >= len(args) {
		return flags, "", false
	}

	// Check if the command matches a known internal subcommand
	firstCmd := args[i]
	if IsInternalSubcommand(firstCmd) {
		return flags, "", false
	}

	// It's a passthrough command!
	cmdStr = ReconstructCommand(args[i:])
	return flags, cmdStr, true
}

// ReconstructCommand reassembles sliced shell arguments into a single safe command string
// using platform-accurate quoting and escaping rules.
func ReconstructCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		if runtime.GOOS == "windows" {
			quoted[i] = QuoteWindowsArg(arg)
		} else {
			quoted[i] = QuoteUnixArg(arg)
		}
	}
	return strings.Join(quoted, " ")
}

// QuoteWindowsArg escapes and quotes an argument according to Windows CommandLineToArgvW rules.
func QuoteWindowsArg(arg string) string {
	if arg == "" {
		return `""`
	}
	needsQuotes := false
	for _, c := range arg {
		if c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '"' ||
			c == '&' || c == '|' || c == '<' || c == '>' || c == '^' || c == '(' || c == ')' {
			needsQuotes = true
			break
		}
	}
	if !needsQuotes {
		return arg
	}

	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for i := 0; i < len(arg); i++ {
		c := arg[i]
		switch c {
		case '\\':
			backslashes++
		case '"':
			for k := 0; k < backslashes*2+1; k++ {
				b.WriteByte('\\')
			}
			b.WriteByte('"')
			backslashes = 0
		default:
			for k := 0; k < backslashes; k++ {
				b.WriteByte('\\')
			}
			b.WriteByte(c)
			backslashes = 0
		}
	}
	for k := 0; k < backslashes*2; k++ {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}

// QuoteUnixArg escapes and quotes an argument according to POSIX shell rules.
func QuoteUnixArg(arg string) string {
	if arg == "" {
		return `''`
	}
	needsQuotes := false
	for _, c := range arg {
		if c == ' ' || c == '\t' || c == '\n' || c == '\'' || c == '"' ||
			c == '&' || c == '|' || c == ';' || c == '<' || c == '>' ||
			c == '$' || c == '`' || c == '\\' || c == '*' || c == '?' ||
			c == '(' || c == ')' || c == '[' || c == ']' || c == '#' || c == '~' {
			needsQuotes = true
			break
		}
	}
	if !needsQuotes {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// RunPassthrough executes a command string through the deterministic msh execution engine
// with real-time output sanitization, truncation, prompt handling, and exit code propagation.
func RunPassthrough(flags PassthroughFlags, command string) {
	session, err := execution.NewSession(flags.Cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[msh] failed to create session: %v\n", err)
		os.Exit(1)
	}

	var redactSecrets *bool
	if flags.NoRedact {
		f := false
		redactSecrets = &f
	}

	req := protocol.ExecRequest{
		Command:        command,
		Cwd:            flags.Cwd,
		Timeout:        flags.Timeout,
		MaxOutputLines: flags.MaxLines,
		DetectFiles:    !flags.NoFiles,
		UsePty:         flags.Pty,
		EnvFile:        flags.EnvFile,
		RedactSecrets:  redactSecrets,
		PromptAnswers:  flags.Answers,
	}

	executor := execution.NewExecutor(session)
	resp := executor.Execute(req)

	if resp.Stdout != "" {
		fmt.Println(resp.Stdout)
	}
	if resp.Stderr != "" {
		fmt.Fprintln(os.Stderr, resp.Stderr)
	}

	switch resp.Status {
	case protocol.StatusBlocked:
		fmt.Fprintf(os.Stderr, "\n[msh] process blocked waiting for interactive prompt: %s\n", resp.PromptDetected)
	case protocol.StatusTimeout:
		fmt.Fprintf(os.Stderr, "\n[msh] process killed due to timeout (%s)\n", flags.Timeout)
	}

	if resp.ExitCode == -1 {
		os.Exit(1)
	}
	os.Exit(resp.ExitCode)
}
