// Package looprun is the application layer for durable verify-until-done loops: starting or resuming a loop,
// recording attempts, stopping it, and reading its status or repo gate. It orchestrates domain/looprun
// rules over a locked store.
package looprun

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/looprun"
	domain "issueops/internal/domain/looprun"
)

type readStore interface {
	ReadAllExisting() ([]domain.LoopObservation, error)
}

// Reader answers read-only loop questions: repository gate readiness and per-loop status lookups.
type Reader struct {
	Store    readStore
	Identity identity
}

func (reader Reader) observeRepo(repo string) domain.RepoObservation {
	observation := domain.RepoObservation{Repo: strings.TrimSpace(repo)}
	if observation.Repo == "" {
		return observation
	}
	normalized, err := reader.Identity.NormalizeRepo(observation.Repo)
	if err != nil {
		observation.ResolveError = err
		return observation
	}
	observation.Repo = normalized
	loops, err := reader.Store.ReadAllExisting()
	if err != nil {
		observation.ListError = err
		return observation
	}
	observation.Loops = loops
	return observation
}

// RepoGateMissing returns the missing loop requirements for repo's gate and any warnings;
// an empty missing list means the gate is satisfied.
func (reader Reader) RepoGateMissing(repo string) ([]string, []string) {
	result := domain.EvaluateRepoGate(reader.observeRepo(repo))
	return result.Missing, result.Warnings
}

// RepoGateSummaryFor returns the repo gate summary for repo together with any warnings.
func (reader Reader) RepoGateSummaryFor(repo string) (contract.RepoGateSummary, []string) {
	result := domain.EvaluateRepoGate(reader.observeRepo(repo))
	return result.Summary, result.Warnings
}

// Status reads the loop with the given ID and returns its status summary. A read failure
// is returned with OK false.
func (service Service) Status(id string) (contract.StatusResult, error) {
	loop, err := service.Store.Read(id)
	if err != nil {
		return contract.StatusResult{OK: false}, err
	}
	return domain.Status(loop), nil
}

// ResolveID derives the deterministic loop ID for a repo and loop name. It fails when the repo
// cannot be normalized or name is blank.
func (service Service) ResolveID(repo, name string) (string, error) {
	repo, err := service.Identity.NormalizeRepo(repo)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	return service.Identity.NewID(repo, name), nil
}
