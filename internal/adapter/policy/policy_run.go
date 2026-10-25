package policy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	policyapp "issueops/internal/application/policy"
	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

func (e Evaluator) RunReadOnly(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return e.service().RunReadOnly(request)
}

func (e Evaluator) service() policyapp.Service {
	return policyapp.Service{Observer: CommandObserver{}, PreparedBaseBranch: e.lookup, Overrides: OverrideLoader{}, Executor: CommandExecutor{}, Clock: Clock{}}
}

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

type CommandExecutor struct{}

func (CommandExecutor) Execute(request policycontract.CommandPolicyRequest, timeout time.Duration) policyapp.Execution {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
	configureProcessGroup(cmd)
	cmd.Dir = request.CWD
	cmd.Env = policydomain.CommandEnvironment(os.Environ(), request.EnvAllowlist)
	// Own both pipes: Cmd.Wait must reap the parent without closing readers or
	// losing buffered output. Completion includes EOF on both inherited pipes.
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		return policyapp.Execution{Err: err, ExitCode: 1}
	}
	defer outRead.Close()
	defer outWrite.Close()
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		return policyapp.Execution{Err: err, ExitCode: 1}
	}
	defer errRead.Close()
	defer errWrite.Close()
	cmd.Stdout, cmd.Stderr = outWrite, errWrite
	if err = ctx.Err(); err != nil {
		return policyapp.Execution{Err: err, ExitCode: 1, TimedOut: errors.Is(err, context.DeadlineExceeded)}
	}
	if err = cmd.Start(); err != nil {
		// A command that never started (a typo in its name) used to surface
		// as "exit code 1: (no output)"; 127 and the reason follow sh.
		exitCode := 1
		if errors.Is(err, exec.ErrNotFound) {
			exitCode = 127
		}
		return policyapp.Execution{Err: err, ExitCode: exitCode, Stderr: "command did not start: " + err.Error() + "\n", TimedOut: ctx.Err() == context.DeadlineExceeded}
	}
	_ = outWrite.Close()
	_ = errWrite.Close()
	return superviseCommand(ctx, cmd, outRead, errRead)
}

// superviseCommand drains both pipes while waiting for the process, enforcing
// the context deadline by terminating the process group.
func superviseCommand(ctx context.Context, cmd *exec.Cmd, outRead, errRead *os.File) policyapp.Execution {
	var stdout, stderr boundedOutput
	copied := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(&stdout, outRead); copied <- struct{}{} }()
	go func() { _, _ = io.Copy(&stderr, errRead); copied <- struct{}{} }()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var err error
	var processDone bool
	streamsDone := 0
	timedOut := false
	deadline := ctx.Done()
	var grace *time.Timer
	var graceDone <-chan time.Time
	for !processDone || streamsDone < 2 {
		select {
		case err = <-waited:
			processDone = true
		case <-copied:
			streamsDone++
		case <-deadline:
			timedOut = true
			deadline = nil
			if terminateErr := terminateProcessGroup(cmd); terminateErr != nil && !errors.Is(terminateErr, os.ErrProcessDone) {
				_ = cmd.Process.Kill()
			}
			// A descendant that escapes the group cannot hold our readers indefinitely.
			grace = time.NewTimer(250 * time.Millisecond)
			graceDone = grace.C
		case <-graceDone:
			_ = outRead.Close()
			_ = errRead.Close()
			graceDone = nil
		}
	}
	if grace != nil {
		grace.Stop()
	}
	if timedOut {
		err = context.DeadlineExceeded
	}
	result := policyapp.Execution{Stdout: stdout.String(), Stderr: stderr.String(), StdoutTruncated: stdout.truncated, StderrTruncated: stderr.truncated, TimedOut: timedOut, Err: err}
	if err != nil {
		result.ExitCode = 1
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	}
	return result
}

// Retain the head and the tail of a stream while accepting all writes so pipe
// readers keep draining. The tail matters: a gate EXPECT usually targets the
// summary line a test runner prints last, which a head-only capture lost
// (probe: ~88 KB of output, then ALL_PASS). Together with the marker the
// result stays under the policy runner's 32 KiB budget.
// Do not embed bytes.Buffer: its ReadFrom would bypass the bounded Write method.
const (
	headCapturedOutputBytes = 20 << 10
	tailCapturedOutputBytes = 12<<10 - 64
)

type boundedOutput struct {
	buffer bytes.Buffer
	// tail grows to twice its cap before it is compacted, so a stream of tiny
	// writes does not copy the whole window on every write.
	tail      []byte
	compacted bool
	truncated bool
	// secret is set when any part of the stream, including the omitted middle,
	// looks like a secret; carry overlaps chunk borders for that scan.
	secret  bool
	carry   []byte
	scratch []byte
	// scanned counts the bytes the secret scan has covered. The scan runs at
	// about 6 MB/s, so it stops at secretScanLimitBytes; a stream longer than
	// that keeps only its head, as before, because its tail was not scanned.
	scanned   int
	unscanned bool
}

const (
	secretScanOverlapBytes = 512
	secretScanLimitBytes   = 4 << 20
)

func (output *boundedOutput) Write(value []byte) (int, error) {
	size := len(value)
	output.scan(value)
	if remaining := headCapturedOutputBytes - output.buffer.Len(); remaining > 0 {
		head := value[:min(remaining, len(value))]
		if _, err := output.buffer.Write(head); err != nil {
			return size, err
		}
		value = value[len(head):]
	}
	if len(value) > 0 {
		output.tail = append(output.tail, value...)
		if len(output.tail) > 2*tailCapturedOutputBytes {
			output.tail = append(output.tail[:0], output.tail[len(output.tail)-tailCapturedOutputBytes:]...)
			output.compacted = true
		}
		output.truncated = output.compacted || len(output.tail) > tailCapturedOutputBytes
	}
	return size, nil
}

// scan keeps the whole-stream secret check the head-only capture had: the
// policy runner redacts the entire output when any part looks like a secret,
// and a signal in the omitted middle must still count (security review).
func (output *boundedOutput) scan(value []byte) {
	if output.secret || output.unscanned {
		return
	}
	if output.scanned += len(value); output.scanned > secretScanLimitBytes {
		output.unscanned = true
		return
	}
	output.scratch = append(append(output.scratch[:0], output.carry...), value...)
	if policydomain.SecretLikeBytes(output.scratch) {
		output.secret = true
		return
	}
	output.carry = append(output.carry[:0], output.scratch[max(0, len(output.scratch)-secretScanOverlapBytes):]...)
}

func (output *boundedOutput) String() string {
	if output.secret {
		if output.truncated {
			return "<redacted>\n<truncated>\n"
		}
		return "<redacted>"
	}
	head := output.buffer.String()
	if !output.truncated {
		return head + string(output.tail)
	}
	// Only complete lines are safe to redact and publish: the unfinished head
	// line and the tail's first line may each hold part of a secret whose
	// identifying rest was discarded.
	head = head[:strings.LastIndexByte(head, '\n')+1]
	if output.unscanned {
		return head + "<truncated>\n"
	}
	window := string(output.tail[max(0, len(output.tail)-tailCapturedOutputBytes):])
	_, tail, found := strings.Cut(window, "\n")
	if !found {
		tail = ""
	}
	return head + "<truncated>\n" + tail
}
