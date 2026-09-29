package looprun

import (
	"fmt"
	"sort"
	"strings"

	contract "issueops/internal/contract/looprun"
)

type LoopObservation struct {
	ID    string
	Loop  contract.LoopRun
	Error error
}
type RepoObservation struct {
	Repo                    string
	ResolveError, ListError error
	Loops                   []LoopObservation
}
type RepoGate struct {
	Missing, Warnings []string
	Summary           contract.RepoGateSummary
}

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
		if Incomplete(loop) {
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
