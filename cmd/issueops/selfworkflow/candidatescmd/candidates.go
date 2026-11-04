// Package candidatescmd implements the `issueops self-verify candidates`
// command, which exports the improvement candidate curriculum.
package candidatescmd

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
	app "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

// Deps injects candidate export, snapshot persistence, and JSON output for Run.
type Deps struct {
	Export    func() contract.SelfVerificationCandidateExportResult
	Save      func(result *contract.SelfVerificationCandidateExportResult, key string) error
	PrintJSON func(any) error
}

// Run parses the candidates flags, exports the candidate list, optionally
// saves it under --state-key, and prints JSON or a text list.
func Run(args []string, deps Deps) error {
	fs := flag.NewFlagSet("self-verify candidates", flag.ContinueOnError)
	saveState := fs.Bool("save-state", false, "save candidate export snapshot to issueops state")
	stateKey := fs.String("state-key", "self-verify-candidates-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.ParseNoArgs(fs, args); err != nil {
		return err
	}
	result, err := app.ExportAndSaveCandidates(*saveState, *stateKey, app.ExportAndSaveCandidatesDeps{Export: deps.Export, Save: deps.Save})
	if err != nil {
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	fmt.Printf("%s candidates: %d candidate(s), selected=%s\n", result.KoreanName, result.CandidateCount, domain.SelectedCandidateID(result.SelectedCandidate))
	for _, candidate := range result.Candidates {
		fmt.Printf("- %s %s score=%.1f status=%s\n", candidate.ID, candidate.Category, candidate.Score, candidate.Status)
	}
	return nil
}
