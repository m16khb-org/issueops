package issueops

import (
	"context"
	"time"

	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

// RemotePublicationObserver gives remote publication preparation its read-only view of the
// cycle record, workspace fingerprint and head. CurrentFingerprint and CurrentHead supply the
// live workspace values; Clock and OperationIDFactory are optional overrides for time and
// operation IDs.
type RemotePublicationObserver struct {
	CurrentFingerprint func(model.Record) string
	CurrentHead        func(model.Record) string
	StateRoot          string
	Clock              func() time.Time
	OperationIDFactory func() (string, error)
}

func (o RemotePublicationObserver) Read(_ context.Context, id string) (model.Record, error) {
	return ReadIssueOps(o.StateRoot, id)
}

// Fingerprint returns the current workspace fingerprint for record.
func (o RemotePublicationObserver) Fingerprint(_ context.Context, record model.Record) string {
	return o.CurrentFingerprint(record)
}

// Head returns the current HEAD commit of the workspace for record.
func (o RemotePublicationObserver) Head(_ context.Context, record model.Record) string {
	return o.CurrentHead(record)
}

var _ application.PreparationObserver = RemotePublicationObserver{}

// PathsMatch reports whether left and right name the same location, resolving symlinks in
// the existing part of each path.
func (o RemotePublicationObserver) PathsMatch(left, right string) bool { return samePath(left, right) }

// Timestamp returns the current time as the execution timestamp string, using Clock when set.
func (o RemotePublicationObserver) Timestamp() string { return executionNow(o.Clock) }

// NewOperationID returns a new external-operation ID, using OperationIDFactory when set.
func (o RemotePublicationObserver) NewOperationID() (string, error) {
	if o.OperationIDFactory != nil {
		return o.OperationIDFactory()
	}
	return newExecutionOperationID()
}

var _ application.PublicationEnvironment = RemotePublicationObserver{}
