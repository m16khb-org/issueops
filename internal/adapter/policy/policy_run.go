package policy

import (
	"bytes"
	"context"
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
		return policyapp.Execution{Err: err, ExitCode: 1, TimedOut: err == context.DeadlineExceeded}
	}
	if err = cmd.Start(); err != nil {
		return policyapp.Execution{Err: err, ExitCode: 1, TimedOut: ctx.Err() == context.DeadlineExceeded}
	}
	_ = outWrite.Close()
	_ = errWrite.Close()
	var stdout, stderr boundedOutput
	copied := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(&stdout, outRead); copied <- struct{}{} }()
	go func() { _, _ = io.Copy(&stderr, errRead); copied <- struct{}{} }()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
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
			if terminateErr := terminateProcessGroup(cmd); terminateErr != nil && terminateErr != os.ErrProcessDone {
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	}
	return result
}

// Retain a fixed prefix while accepting all writes so pipe readers keep draining.
// Do not embed bytes.Buffer: its ReadFrom would bypass the bounded Write method.
const maxCapturedOutputBytes = 64 << 10

type boundedOutput struct {
	buffer    bytes.Buffer
	truncated bool
}

func (output *boundedOutput) Write(value []byte) (int, error) {
	size := len(value)
	remaining := maxCapturedOutputBytes - output.buffer.Len()
	if len(value) > remaining {
		value = value[:remaining]
		output.truncated = true
	}
	_, err := output.buffer.Write(value)
	return size, err
}

func (output *boundedOutput) String() string {
	value := output.buffer.String()
	if output.truncated {
		// The unfinished line may contain the start of a secret whose identifying
		// suffix was discarded. Only complete lines are safe to redact and publish.
		value = value[:strings.LastIndexByte(value, '\n')+1]
	}
	return value
}
