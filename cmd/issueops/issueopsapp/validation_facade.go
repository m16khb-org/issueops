package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/internal/adapter/verification"
	selfverifyapp "issueops/internal/application/selfverify"
)

func validateGoFormat(root string) selfverify.StepResult {
	return selfverifyapp.ValidateFormat(root, selfverifyapp.FormatDeps{
		ListTrackedGoFiles: verification.ListTrackedGoFiles,
		ListUnformatted:    verification.ListUnformatted,
		Now:                time.Now,
	})
}
