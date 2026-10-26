package providerutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"issueops/internal/adapter/outbound/processlease"
	"issueops/internal/domain/policy"
	"issueops/internal/domain/shelltoken"
)

const (
	providerReadbackTimeout = 15 * time.Second
	providerMutationTimeout = 60 * time.Second
	providerReadbackLimit   = 256 * 1024
	providerDiagnosticLimit = 4096
)

func RunBoundedReadbackContext(ctx context.Context, repo, name string, args ...string) ([]byte, error) {
	stdout, _, err := runBoundedCommandContext(ctx, repo, name, args, providerReadbackTimeout, providerReadbackLimit)
	return stdout, err
}

func RunBoundedMutationContext(ctx context.Context, repo, name string, args ...string) (stdout []byte, invoked bool, err error) {
	return runBoundedCommandContext(ctx, repo, name, args, providerMutationTimeout, providerReadbackLimit)
}

func RunBoundedMutationWithOutputLimitContext(ctx context.Context, repo, name string, outputLimit int, args ...string) (stdout []byte, invoked bool, err error) {
	return runBoundedCommandContext(ctx, repo, name, args, providerMutationTimeout, outputLimit)
}

// DryRunPreview renders the command a confirm would run, quoting each word
// that a shell would split or expand, so a title like `hello.sh --version`
// does not read as a flag. A --body/--description value is shown as its
// length: the caller wrote that body and the readability report checks it,
// and the shell-quoted copy (up to the 4 KB cap, often cut) repeated it in
// every preview (opencode QA read it as a broken body).
func DryRunPreview(name string, args ...string) string {
	argv := policy.RedactArgv(append([]string{name}, args...))
	for i, word := range argv {
		if i > 0 && (argv[i-1] == "--body" || argv[i-1] == "--description") && word != "-" {
			word = fmt.Sprintf("<body: %d chars>", utf8.RuneCountInString(args[i-1]))
		}
		if word == "" || strings.ContainsFunc(word, needsShellQuote) {
			word = shelltoken.QuoteWord(word)
		}
		argv[i] = word
	}
	value := strings.Join(argv, " ")
	if len(value) > providerDiagnosticLimit {
		value = policy.TruncateBytes(value, providerDiagnosticLimit) + "...[truncated]"
	}
	return "[dry-run] would execute: " + value
}

func needsShellQuote(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("_@%+=:,./-", r)
}

func runBoundedCommandContext(parent context.Context, repo, name string, args []string, timeout time.Duration, outputLimit int) ([]byte, bool, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	processlease.Attach(ctx, cmd)
	cmd.Dir = repo
	stdout := &boundedBuffer{limit: outputLimit}
	stderr := &boundedBuffer{limit: providerDiagnosticLimit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("command start failed: %s", boundedDiagnostic(err.Error()))
	}
	err := cmd.Wait()
	output := append([]byte(nil), stdout.data...)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return output, true, fmt.Errorf("command failed after start: %w", ctxErr)
	}
	if stdout.truncated {
		return output, true, fmt.Errorf("command output exceeds %d bytes", outputLimit)
	}
	if err != nil {
		return output, true, fmt.Errorf("command failed after start: %s", boundedDiagnostic(stderr.String()+" "+err.Error()))
	}
	return output, true, nil
}

type boundedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	if n > remaining {
		b.truncated = true
	}
	return n, nil
}

func (b *boundedBuffer) String() string {
	if b.truncated {
		return policy.TrimIncompleteRune(string(b.data))
	}
	return string(b.data)
}

func boundedDiagnostic(value string) string {
	return policy.BoundedDiagnostic(value, providerDiagnosticLimit)
}
