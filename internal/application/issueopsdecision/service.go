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

type Service struct {
	repository Repository
	clock      Clock
	paths      PathMatcher
	verifier   authorityport.ActorVerifier
}

func NewService(repository Repository, clock Clock, paths PathMatcher, verifier authorityport.ActorVerifier) *Service {
	return &Service{repository: repository, clock: clock, paths: paths, verifier: verifier}
}

func (service *Service) Add(
	ctx context.Context,
	stateRoot string,
	id string,
	request issueopsdecisioncontract.Request,
	actor *issueopscontract.IssueOpsActor,
) (issueopscontract.IssueOpsRecord, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf(
			"issueops decision dependencies are required",
		)
	}
	decision, err := issueopsdecisiondomain.Build(request, service.clock.Now())
	if err != nil {
		return issueopscontract.IssueOpsRecord{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopscontract.IssueOpsRecord,
	) (issueopscontract.IssueOpsRecord, error) {
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
