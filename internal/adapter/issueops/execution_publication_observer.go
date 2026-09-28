package issueops

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops/implementation"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type RemotePublicationObserver struct {
	StateRoot          string
	Clock              func() time.Time
	OperationIDFactory func() (string, error)
}

func (o RemotePublicationObserver) NormalizeActor(_ context.Context, actor contract.Actor) (contract.Actor, error) {
	normalized, err := normalizeNativeActor(publicationActor(actor))
	if err != nil {
		return contract.Actor{}, err
	}
	result := actor.Clone()
	result.Host, result.SessionID, result.AgentID = normalized.Host, normalized.SessionID, normalized.AgentID
	if normalized.SessionProcess != nil {
		receipt := contract.ProcessReceipt(*normalized.SessionProcess)
		result.SessionProcess = &receipt
	}
	return result, nil
}

func (o RemotePublicationObserver) Read(_ context.Context, id string) (model.IssueOpsRecord, error) {
	return ReadIssueOps(o.StateRoot, id)
}

func (o RemotePublicationObserver) Authorize(_ context.Context, record model.IssueOpsRecord, command contract.CreateCommand) error {
	actor := publicationActor(command.Actor)
	return validateExecutionMutation(record, &IssueOpsActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: command.CWD, NativeProcessAncestry: actor.ProcessAncestry})
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
