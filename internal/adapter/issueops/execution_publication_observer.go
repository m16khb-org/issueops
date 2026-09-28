package issueops

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops/implementation"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

type RemotePublicationObserver struct {
	StateRoot          string
	Clock              func() time.Time
	OperationIDFactory func() (string, error)
}

func (o RemotePublicationObserver) Read(_ context.Context, id string) (model.IssueOpsRecord, error) {
	return ReadIssueOps(o.StateRoot, id)
}

func (o RemotePublicationObserver) Fingerprint(_ context.Context, record model.IssueOpsRecord) string {
	return implementation.ChangeFingerprint(record)
}
func (o RemotePublicationObserver) Head(_ context.Context, record model.IssueOpsRecord) string {
	return issueOpsCurrentHead(record)
}

var _ application.PreparationObserver = RemotePublicationObserver{}

func (o RemotePublicationObserver) PathsMatch(left, right string) bool { return samePath(left, right) }
func (o RemotePublicationObserver) Timestamp() string                  { return executionNow(o.Clock) }
func (o RemotePublicationObserver) NewOperationID() (string, error) {
	if o.OperationIDFactory != nil {
		return o.OperationIDFactory()
	}
	return newExecutionOperationID()
}

var _ application.PublicationEnvironment = RemotePublicationObserver{}
