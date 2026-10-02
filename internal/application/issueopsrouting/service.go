package issueopsrouting

import (
	"context"
	"fmt"

	cycleapp "issueops/internal/application/issueopscycle"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
	issueopsroutingdomain "issueops/internal/domain/issueopsrouting"
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

func (service *Service) Record(
	ctx context.Context,
	stateRoot string,
	id string,
	phase string,
	skill string,
	actor issueopsroutingcontract.Actor,
) (issueopsroutingcontract.Record, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopsroutingcontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops routing dependencies are required",
		)
	}
	entry, err := issueopsroutingdomain.NewEntry(phase, skill, service.clock.Now())
	if err != nil {
		return issueopsroutingcontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsroutingcontract.Record,
	) (issueopsroutingcontract.Record, bool, error) {
		if err := cycleapp.AuthorizeHolder(
			ctx,
			record,
			&actor,
			service.paths.Same,
			service.verifier,
		); err != nil {
			record.OK = false
			return record, false, err
		}
		return issueopsroutingdomain.Append(record, entry)
	})
}

func (service *Service) Score(
	ctx context.Context,
	stateRoot string,
	id string,
	expected []issueopsroutingcontract.Expected,
) (issueopsroutingcontract.Result, int, error) {
	if service == nil || service.repository == nil {
		return issueopsroutingcontract.Result{OK: false}, 0, fmt.Errorf(
			"issueops routing repository is required",
		)
	}
	record, err := service.repository.Read(ctx, stateRoot, id)
	if err != nil {
		return issueopsroutingcontract.Result{OK: false}, 0, err
	}
	return issueopsroutingdomain.ScoreRecord(record, expected), len(record.RoutingTrace), nil
}
