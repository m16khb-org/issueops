// Package issueopsrouting is the application service that records which skill
// a cycle routed to at each phase and scores that routing trace against the
// expected routes. It depends on the Repository port, a clock, a path matcher,
// and an actor verifier, never on storage directly.
package issueopsrouting

import (
	"context"
	"fmt"
	issueopscontract "issueops/internal/contract/issueops"

	cycleapp "issueops/internal/application/issueopscycle"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
	issueopsroutingdomain "issueops/internal/domain/issueopsrouting"
	authorityport "issueops/internal/port/authority"
)

// Service records and scores the routing trace of an issueops cycle. Build it
// with NewService; its methods fail when required dependencies are nil.
type Service struct {
	repository Repository
	clock      clock
	paths      pathMatcher
	verifier   authorityport.ActorVerifier
}

// NewService returns a Service over repository, clock, and paths, with verifier checking the acting holder.
func NewService(repository Repository, clock clock, paths pathMatcher, verifier authorityport.ActorVerifier) *Service {
	return &Service{repository: repository, clock: clock, paths: paths, verifier: verifier}
}

// Record appends a routing entry for phase and skill to the cycle id's record
// after checking that actor holds the cycle's lease. The entry is timestamped
// by the service clock. It returns an error when dependencies are missing, the
// phase/skill pair is invalid, or the actor is not the holder.
func (service *Service) Record(
	ctx context.Context,
	stateRoot string,
	id string,
	phase string,
	skill string,
	actor issueopscontract.Actor,
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

// Score compares the routing trace stored for cycle id with expected and
// returns the scored result together with the number of recorded trace entries.
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
