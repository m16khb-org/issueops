package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

// SameChangeObservationIdentity reports whether two observations describe the same
// change root. The prepared base SHA and branch only count when HasPreparedBase is set;
// surrounding whitespace is ignored.
func SameChangeObservationIdentity(a, b reviewcontract.ChangeObservationIdentity) bool {
	return changeObservationKey(a) == changeObservationKey(b)
}

func changeObservationKey(identity reviewcontract.ChangeObservationIdentity) string {
	fields := []string{strings.TrimSpace(identity.Root)}
	if identity.HasPreparedBase {
		fields = append(fields, strings.TrimSpace(identity.BaseSHA), strings.TrimSpace(identity.BaseBranch))
	}
	return strings.Join(fields, "\x00")
}
