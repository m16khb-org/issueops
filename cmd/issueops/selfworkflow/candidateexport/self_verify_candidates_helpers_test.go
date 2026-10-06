package candidateexport

import (
	"time"

	app "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
)

// Source observation is installed by the composition root.
var ObserveSource func(string) (string, bool)

func ExportSelfVerificationCandidates(root string) augmentcontract.SelfVerificationCandidateExportResult {
	return app.ExportCandidates(root, app.ExportCandidatesDeps{Source: ObserveSource, Now: time.Now})
}
