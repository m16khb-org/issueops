package looprun

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	loopcontract "issueops/internal/contract/looprun"
)

const (
	defaultMaxAttempts = 5
	maxMaxAttempts     = 50
)

type Start struct {
	Name        string
	Goal        string
	VerifyArgv  []string
	MaxAttempts int
}

type Attempt struct {
	verdict  string
	evidence []string
}

var secretAssignmentPattern = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(token|secret|password|api[_-]?key|access[_-]?key)\s*[:=]\s*["']?([^\s"',}]+)`)
})

func PrepareStart(request loopcontract.StartLoopRequest) (Start, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return Start{}, fmt.Errorf("name is required")
	}
	goal := redactFreeform(request.Goal)
	if goal == "" {
		return Start{}, fmt.Errorf("goal is required")
	}
	maxAttempts := request.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	} else if maxAttempts < 0 || maxAttempts > maxMaxAttempts {
		return Start{}, fmt.Errorf("max_attempts_invalid")
	}
	return Start{Name: name, Goal: goal, VerifyArgv: cleanStrings(request.VerifyArgv), MaxAttempts: maxAttempts}, nil
}

func New(id, repo string, request Start, at string, schemaVersion int) loopcontract.LoopRun {
	return loopcontract.LoopRun{
		OK: true, SchemaVersion: schemaVersion, ID: id, Repo: repo, Name: request.Name,
		Goal: request.Goal, VerifyArgv: append([]string(nil), request.VerifyArgv...),
		MaxAttempts: request.MaxAttempts, Status: "active", CreatedAt: at, UpdatedAt: at,
	}
}

func Resume(existing loopcontract.LoopRun) error {
	if existing.Status != "active" {
		// MCP QA: a bare "loop_terminal" left the agent guessing the loop id
		// and what to do; name both.
		return fmt.Errorf("loop_terminal: loop %s (%s) is already %s; start a new loop under another name, or read this one with loop status --id %s", existing.ID, existing.Name, existing.Status, existing.ID)
	}
	return nil
}

func PrepareAttempt(request loopcontract.RecordAttemptRequest) (Attempt, error) {
	verdict := strings.TrimSpace(request.Verdict)
	if verdict != "pass" && verdict != "fail" {
		return Attempt{}, errVerdictInvalid
	}
	evidence := cleanStrings(request.Evidence)
	if len(evidence) == 0 {
		return Attempt{}, errEvidenceRequired
	}
	for i := range evidence {
		evidence[i] = redactFreeform(evidence[i])
	}
	return Attempt{verdict: verdict, evidence: evidence}, nil
}

// Probe (MCP loop_record_attempt verdict "maybe"): bare codes named no valid
// value. Each code stays the message prefix.
var (
	errVerdictInvalid   = fmt.Errorf("verdict_invalid: verdict must be pass or fail")
	errEvidenceRequired = fmt.Errorf("evidence_required: pass the output you saw when you ran the verify command")
)

func ApplyAttempt(current loopcontract.LoopRun, attempt Attempt, at string) (loopcontract.LoopRun, error) {
	if current.Status != "active" {
		return loopcontract.LoopRun{}, fmt.Errorf("loop_not_active: loop %s is %s; attempts are recorded only on an active loop (start a new loop under another name)", current.ID, current.Status)
	}
	if attempt.verdict != "pass" && attempt.verdict != "fail" {
		return loopcontract.LoopRun{}, errVerdictInvalid
	}
	if len(attempt.evidence) == 0 {
		return loopcontract.LoopRun{}, errEvidenceRequired
	}
	next := current
	next.Attempts = append(append([]loopcontract.LoopAttempt(nil), current.Attempts...), loopcontract.LoopAttempt{
		Seq: len(current.Attempts) + 1, Verdict: attempt.verdict,
		Evidence: append([]string(nil), attempt.evidence...), At: at,
	})
	if attempt.verdict == "fail" && len(next.Attempts) >= next.MaxAttempts {
		next.Status = "exhausted"
	}
	next.UpdatedAt = at
	return next, nil
}

func Stop(current loopcontract.LoopRun, success bool, reason, at string) (loopcontract.LoopRun, error) {
	if current.Status == "succeeded" || current.Status == "stopped" {
		return loopcontract.LoopRun{}, fmt.Errorf("loop_terminal")
	}
	next := current
	if success {
		if len(current.Attempts) == 0 || current.Attempts[len(current.Attempts)-1].Verdict != "pass" {
			return loopcontract.LoopRun{}, fmt.Errorf("loop_success_requires_pass: the latest attempt must be a pass; record one with loop record-attempt --verdict pass, or stop with --reason TEXT")
		}
		next.Status = "succeeded"
	} else {
		reason = redactFreeform(reason)
		if len(reason) < 10 {
			return loopcontract.LoopRun{}, fmt.Errorf("stop_reason_too_short: a non-success stop needs a reason of at least 10 characters")
		}
		next.Status = "stopped"
		next.StopReason = reason
	}
	next.UpdatedAt = at
	return next, nil
}

func Incomplete(loop loopcontract.LoopRun) bool {
	switch strings.TrimSpace(loop.Status) {
	case "active", "exhausted":
		return true
	default:
		return false
	}
}

func Status(loop loopcontract.LoopRun) loopcontract.StatusResult {
	result := loopcontract.StatusResult{OK: true, Loop: loop, AttemptCount: len(loop.Attempts)}
	if len(loop.Attempts) > 0 {
		result.LastVerdict = loop.Attempts[len(loop.Attempts)-1].Verdict
	}
	return result
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func redactFreeform(value string) string {
	return strings.TrimSpace(secretAssignmentPattern().ReplaceAllString(value, "$1=<redacted>"))
}
