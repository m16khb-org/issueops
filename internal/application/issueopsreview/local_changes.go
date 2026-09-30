package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsreview"
	domain "issueops/internal/domain/issueopsreview"
	port "issueops/internal/port/issueopsreview"
	"strings"
)

type LocalChangeObserver struct{ Source port.LocalChangeSource }

// Observe reads paths and contents twice, allowing one retry for a concurrent update.
// An unstable or unreadable snapshot never carries a verified fingerprint.
func (o LocalChangeObserver) Observe(record model.IssueOpsRecord, root string) contract.LocalChangeObservation {
	if strings.TrimSpace(root) == "" || o.Source.Fingerprint == nil {
		return contract.LocalChangeObservation{}
	}
	base := o.Source.BaseRef(record, root)
	var lastPaths []string
	for range 2 {
		first, ok := o.Source.Paths(root, base)
		if !ok {
			return contract.LocalChangeObservation{Paths: lastPaths}
		}
		lastPaths = first
		firstFingerprint, ok := o.Source.Fingerprint(root, first)
		if !ok {
			continue
		}
		second, ok := o.Source.Paths(root, base)
		if !ok {
			continue
		}
		lastPaths = second
		secondFingerprint, ok := o.Source.Fingerprint(root, second)
		if ok && domain.SameChangeSnapshot(first, second, firstFingerprint, secondFingerprint) {
			return contract.LocalChangeObservation{Paths: append([]string{}, first...), Fingerprint: firstFingerprint, Verified: true}
		}
	}
	return contract.LocalChangeObservation{Paths: append([]string{}, lastPaths...)}
}
