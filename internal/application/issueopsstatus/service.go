package issueopsstatus

import (
	"context"
	"fmt"

	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

// Service reads a cycle record from a Repository and projects it into the
// status result.
type Service struct {
	repository Repository
	projector  projector
}

// NewService returns a Service that reads records through repository and
// derives the status view with projector.
func NewService(repository Repository, projector projector) *Service {
	return &Service{repository: repository, projector: projector}
}

// Status returns the projected status of cycle id under stateRoot. A read error
// is returned with the record the repository produced; missing dependencies
// return an error and a record with OK false.
func (service *Service) Status(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsstatuscontract.Record, error) {
	if service == nil || service.repository == nil || service.projector == nil {
		return issueopsstatuscontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops status dependencies are required",
		)
	}
	record, err := service.repository.Read(ctx, stateRoot, id)
	if err != nil {
		return record, err
	}
	return service.projector.Project(record), nil
}
