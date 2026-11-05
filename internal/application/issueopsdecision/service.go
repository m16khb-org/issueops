package issueopsdecision

import (
	"context"
	"fmt"
	issueopscontract "issueops/internal/contract/issueops"

	cycleapp "issueops/internal/application/issueopscycle"
	issueopsdecisioncontract "issueops/internal/contract/issueopsdecision"
	issueopsdecisiondomain "issueops/internal/domain/issueopsdecision"
	authorityport "issueops/internal/port/authority"
)

// Service adds decisions to lifecycle records. Construct it with NewService.
type Service struct {
	repository Repository
	clock      clock
	paths      pathMatcher
	verifier   authorityport.ActorVerifier
}

// NewService returns a Service using repository, clock, the path matcher, and the
// actor verifier for holder authorization. Add fails if repository, clock, or
// path matcher is nil.
func NewService(repository Repository, clock clock, paths pathMatcher, verifier authorityport.ActorVerifier) *Service {
	return &Service{repository: repository, clock: clock, paths: paths, verifier: verifier}
}

// Add builds a decision from request, stamped with the service clock, and appends
// it to the record with id after confirming that actor holds the cycle. The
// record is not changed when the request or authorization is rejected.
func (service *Service) Add(
	ctx context.Context,
	stateRoot string,
	id string,
	request issueopsdecisioncontract.Request,
	actor *issueopscontract.Actor,
) (issueopscontract.Record, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopscontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops decision dependencies are required",
		)
	}
	decision, err := issueopsdecisiondomain.Build(request, service.clock.Now())
	if err != nil {
		return issueopscontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopscontract.Record,
	) (issueopscontract.Record, error) {
		if err := cycleapp.AuthorizeHolder(
			ctx,
			record,
			actor,
			service.paths.Same,
			service.verifier,
		); err != nil {
			record.OK = false
			return record, err
		}
		record.Decisions = append(record.Decisions, decision)
		record.UpdatedAt = decision.CreatedAt
		record.OK = true
		return record, nil
	})
}
