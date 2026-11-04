// Package progress reports self-verify progress as JSON Lines events on a writer
// so a long run shows which step it is in and what last succeeded. It is part
// of the cmd layer's self workflow.
package progress

import (
	selfverify "issueops/internal/contract/selfverify"

	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// NewSelfVerifyProgressReporter returns a reporter for mode. An empty mode or
// "none" returns a nil reporter, which every method accepts and treats as
// disabled; "jsonl" writes one JSON event per line to writer (io.Discard when
// nil); any other mode is an error.
func NewSelfVerifyProgressReporter(mode string, writer io.Writer) (*SelfVerifyProgressReporter, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" || mode == "none" {
		return nil, nil
	}
	if mode != "jsonl" {
		return nil, fmt.Errorf("unsupported self-verify progress mode %q; use none or jsonl", mode)
	}
	if writer == nil {
		writer = io.Discard
	}
	return &SelfVerifyProgressReporter{mode: mode, writer: writer, started: time.Now()}, nil
}

// Emit writes event as one JSON line. A zero ElapsedMS is filled from the time
// since the reporter started and an empty LastSuccess from the last successful
// step. A nil reporter does nothing.
func (r *SelfVerifyProgressReporter) Emit(event selfverify.ProgressEvent) {
	if r == nil || r.mode == "" {
		return
	}
	if event.ElapsedMS == 0 {
		event.ElapsedMS = time.Since(r.started).Milliseconds()
	}
	if event.LastSuccess == "" {
		event.LastSuccess = r.lastSuccess
	}
	b, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(r.writer, `{"event":"progress_error","error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(r.writer, string(b))
}

// EmitStepEnd writes a step_end event for step with its loop position and
// duration, then remembers the step as the last success when it passed. A nil
// reporter does nothing.
func (r *SelfVerifyProgressReporter) EmitStepEnd(loopKind string, iteration, iterations int, seed int64, stepIndex, stepCount int, step selfverify.StepResult) {
	if r == nil {
		return
	}
	lastSuccess := r.lastSuccess
	if step.OK {
		lastSuccess = step.Label
	}
	event := selfverify.ProgressEvent{
		Event:       "step_end",
		LoopKind:    loopKind,
		Iteration:   iteration,
		Iterations:  iterations,
		Seed:        seed,
		StepIndex:   stepIndex,
		StepCount:   stepCount,
		Step:        step.Label,
		OK:          new(step.OK),
		DurationMS:  step.DurationMS,
		LastSuccess: lastSuccess,
		Error:       step.Error,
	}
	r.Emit(event)
	if step.OK {
		r.lastSuccess = step.Label
	}
}

// SetStarted sets the instant that elapsed time is measured from, replacing the
// time the reporter was created. A nil reporter does nothing.
func (r *SelfVerifyProgressReporter) SetStarted(started time.Time) {
	if r == nil {
		return
	}
	r.started = started
}
