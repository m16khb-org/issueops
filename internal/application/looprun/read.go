package looprun

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/looprun"
	domain "issueops/internal/domain/looprun"
)

type ReadStore interface {
	ListIDs() ([]string, error)
	ReadExisting(string) (contract.LoopRun, error)
}
type Reader struct {
	Store    ReadStore
	Identity Identity
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
	ids, err := reader.Store.ListIDs()
	if err != nil {
		observation.ListError = err
		return observation
	}
	for _, id := range ids {
		loop, err := reader.Store.ReadExisting(id)
		observation.Loops = append(observation.Loops, domain.LoopObservation{ID: id, Loop: loop, Error: err})
	}
	return observation
}
func (reader Reader) RepoGateMissing(repo string) ([]string, []string) {
	result := domain.EvaluateRepoGate(reader.observeRepo(repo))
	return result.Missing, result.Warnings
}
func (reader Reader) RepoGateSummaryFor(repo string) (contract.RepoGateSummary, []string) {
	result := domain.EvaluateRepoGate(reader.observeRepo(repo))
	return result.Summary, result.Warnings
}
func (service Service) Status(id string) (contract.StatusResult, error) {
	loop, err := service.Store.Read(id)
	if err != nil {
		return contract.StatusResult{OK: false}, err
	}
	return domain.Status(loop), nil
}
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
