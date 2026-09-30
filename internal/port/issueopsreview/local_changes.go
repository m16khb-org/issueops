package issueopsreview

import model "issueops/internal/contract/issueops"

type LocalChangeSource struct {
	BaseRef     func(model.IssueOpsRecord, string) string
	Paths       func(string, string) ([]string, bool)
	Fingerprint func(string, []string) (string, bool)
}
