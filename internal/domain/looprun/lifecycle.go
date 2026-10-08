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
		return fmt.Errorf("loop_terminal")
	}
	return nil
}

func PrepareAttempt(request loopcontract.RecordAttemptRequest) (Attempt, error) {
	verdict := strings.TrimSpace(request.Verdict)
	if verdict != "pass" && verdict != "fail" {
		return Attempt{}, fmt.Errorf("verdict_invalid")
	}
	evidence := cleanStrings(request.Evidence)
	if len(evidence) == 0 {
		return Attempt{}, fmt.Errorf("evidence_required")
	}
	for i := range evidence {
		evidence[i] = redactFreeform(evidence[i])
	}
	return Attempt{verdict: verdict, evidence: evidence}, nil
}

func ApplyAttempt(current loopcontract.LoopRun, attempt Attempt, at string) (loopcontract.LoopRun, error) {
	if current.Status != "active" {
		return loopcontract.LoopRun{}, fmt.Errorf("loop_not_active")
	}
	if attempt.verdict != "pass" && attempt.verdict != "fail" {
		return loopcontract.LoopRun{}, fmt.Errorf("verdict_invalid")
	}
	if len(attempt.evidence) == 0 {
		return loopcontract.LoopRun{}, fmt.Errorf("evidence_required")
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
			return loopcontract.LoopRun{}, fmt.Errorf("loop_success_requires_pass")
		}
		next.Status = "succeeded"
	} else {
		reason = redactFreeform(reason)
		if len(reason) < 10 {
			return loopcontract.LoopRun{}, fmt.Errorf("stop_reason_too_short")
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
