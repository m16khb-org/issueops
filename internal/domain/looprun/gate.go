package looprun

import (
	"fmt"
	"sort"
	"strings"

	contract "issueops/internal/contract/looprun"
)

// LoopObservation is one loop run read from the store together with the error, if any, hit while reading it.
type LoopObservation struct {
	ID    string
	Loop  contract.LoopRun
	Error error
}

// RepoObservation is what the repo gate saw when scanning one repository: the resolve and list errors and every loop run found.
type RepoObservation struct {
	Repo                    string
	ResolveError, ListError error
	Loops                   []LoopObservation
}

// RepoGate is the gate verdict for a repository: Missing criteria that block completion, Warnings to show, and a Summary of active and exhausted loops.
type RepoGate struct {
	Missing, Warnings []string
	Summary           contract.RepoGateSummary
}

// EvaluateRepoGate turns a RepoObservation into a RepoGate. A blank Repo yields an empty gate. A resolve or list error, or an unreadable loop, yields a missing criterion plus a warning. Otherwise every active or exhausted loop of that repo is reported as loop_incomplete:<id>, in sorted order.
func EvaluateRepoGate(observation RepoObservation) RepoGate {
	if strings.TrimSpace(observation.Repo) == "" {
		return RepoGate{}
	}
	if observation.ResolveError != nil {
		return RepoGate{Missing: []string{"loops_complete"}, Warnings: []string{"failed to resolve loop repo: " + observation.ResolveError.Error()}}
	}
	if observation.ListError != nil {
		return RepoGate{Missing: []string{"loops_complete"}, Warnings: []string{"failed to scan loop runs: " + observation.ListError.Error()}}
	}
	result := RepoGate{Missing: []string{}, Warnings: []string{}}
	for _, observed := range observation.Loops {
		if observed.Error != nil {
			result.Missing = append(result.Missing, "loop_incomplete:"+observed.ID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to inspect loop run %s: %v", observed.ID, observed.Error))
			continue
		}
		loop := observed.Loop
		if strings.TrimSpace(loop.Repo) != observation.Repo {
			continue
		}
		if incomplete(loop) {
			result.Missing = append(result.Missing, "loop_incomplete:"+loop.ID)
		}
		switch strings.TrimSpace(loop.Status) {
		case "active":
			result.Summary.Active++
		case "exhausted":
			result.Summary.Exhausted++
		}
	}
	sort.Strings(result.Missing)
	return result
}
