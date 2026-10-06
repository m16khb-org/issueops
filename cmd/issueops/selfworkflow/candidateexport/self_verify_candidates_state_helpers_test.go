package candidateexport

import (
	"encoding/json"
	app "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	"time"
)

func SaveSelfVerificationCandidateExport(result *contract.SelfVerificationCandidateExportResult, key string) error {
	return app.SaveCandidateExport(result, key, app.SaveCandidateExportDeps{
		Now: time.Now,
		Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write:    StateWrite,
		StateDir: StateDir,
	})
}
