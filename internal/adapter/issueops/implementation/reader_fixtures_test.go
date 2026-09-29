package implementation

import (
	preflight "issueops/internal/adapter/preflight"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsreview"
	port "issueops/internal/port/issueopsreview"
	"os"
)

func testReader() Reader { return Reader{GitCmd: preflight.GitCmd, GitCmdRaw: preflight.GitCmdRaw} }
func observeLocalChangesAt(record model.IssueOpsRecord, root string, readFile func(string) ([]byte, error)) contract.LocalChangeObservation {
	reader := testReader()
	var fingerprint func(string, []string) (string, bool)
	if readFile != nil {
		fingerprint = func(root string, paths []string) (string, bool) { return fingerprintPaths(root, paths, readFile) }
	}
	return (app.LocalChangeObserver{Source: port.LocalChangeSource{BaseRef: reader.DiffBaseRef, Paths: reader.ObservedPathsIn, Fingerprint: fingerprint}}).Observe(record, root)
}
func ObserveLocalChangesAt(record model.IssueOpsRecord, root string) contract.LocalChangeObservation {
	return observeLocalChangesAt(record, root, os.ReadFile)
}
