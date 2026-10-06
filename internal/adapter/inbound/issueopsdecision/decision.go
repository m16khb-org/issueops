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
	) (issueopscontract.IssueOpsRecord, error)
	AddWithActor func(
		string,
		string,
		issueopsdecisioncontract.Request,
		issueopscontract.IssueOpsActor,
	) (issueopscontract.IssueOpsRecord, error)
}

func NewHandlers(service *issueopsdecisionapplication.Service) Handlers {
	return Handlers{
		Add: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
		) (issueopscontract.IssueOpsRecord, error) {
			return service.Add(context.Background(), stateRoot, id, request, nil)
		},
		AddWithActor: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
			actor issueopscontract.IssueOpsActor,
		) (issueopscontract.IssueOpsRecord, error) {
			return service.Add(context.Background(), stateRoot, id, request, &actor)
		},
	}
}
