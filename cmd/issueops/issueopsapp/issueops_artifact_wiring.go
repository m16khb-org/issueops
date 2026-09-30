package issueopsapp

import (
	issueopsartifactinbound "issueops/internal/adapter/inbound/issueopsartifact"
	issueopsartifactoutbound "issueops/internal/adapter/outbound/issueopsartifact"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsartifactapplication "issueops/internal/application/issueopsartifact"
)

func issueOpsArtifactHandlers(
	observers ...issueopsrecord.Observer,
) issueopsartifactinbound.Handlers {
	service := issueopsartifactapplication.NewService(issueopsartifactoutbound.Repository{
		Store: issueOpsRecordStore("artifact", observers...),
	})
	return issueopsartifactinbound.NewHandlers(service)
}
