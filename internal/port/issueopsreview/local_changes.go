package issueopsreview

import model "issueops/internal/contract/issueops"

// LocalChangeSource supplies the observations needed to fingerprint a record's local changes:
// the base ref to diff against, the changed paths, and a fingerprint of those paths.
// The bool results report whether the observation was possible.
type LocalChangeSource struct {
	BaseRef     func(model.Record, string) string
	Paths       func(string, string) ([]string, bool)
	Fingerprint func(string, []string) (string, bool)
}
