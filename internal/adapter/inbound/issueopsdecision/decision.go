package issueopsdecision

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"

	issueopsdecisionapplication "issueops/internal/application/issueopsdecision"
	issueopsdecisioncontract "issueops/internal/contract/issueopsdecision"
)

type Handlers struct {
	Add func(
		string,
		string,
		issueopsdecisioncontract.Request,
	) (issueopscontract.Record, error)
	AddWithActor func(
		string,
		string,
		issueopsdecisioncontract.Request,
		issueopscontract.Actor,
	) (issueopscontract.Record, error)
}

func NewHandlers(service *issueopsdecisionapplication.Service) Handlers {
	return Handlers{
		Add: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
		) (issueopscontract.Record, error) {
			return service.Add(context.Background(), stateRoot, id, request, nil)
		},
		AddWithActor: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
			actor issueopscontract.Actor,
		) (issueopscontract.Record, error) {
			return service.Add(context.Background(), stateRoot, id, request, &actor)
		},
	}
}
