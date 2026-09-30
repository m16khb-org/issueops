package issueopsapp

import (
	issueopsstatusinbound "issueops/internal/adapter/inbound/issueopsstatus"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsstatusoutbound "issueops/internal/adapter/outbound/issueopsstatus"
	issueopsstatusapplication "issueops/internal/application/issueopsstatus"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
	issueopsstatusdomain "issueops/internal/domain/issueopsstatus"
)

func issueOpsStatusHandler(
	observers ...issueopsrecord.Observer,
) func(
	string,
	string,
) (issueopsstatuscontract.Record, error) {
	service := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{
			Store: issueOpsRecordStore("status", observers...),
		},
		issueopsstatusdomain.NewProjector(newCycleReadiness().Completion),
	)
	return issueopsstatusinbound.NewStatusHandler(service)
}
