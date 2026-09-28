package issueops

import (
	"encoding/json"

	model "issueops/internal/contract/issueops"
	preparation "issueops/internal/contract/issueopspreparation"
)

type OrcaIntentFiles struct{}

func PreparationIntentRecord(record model.IssueOpsRecord) (preparation.Record, error) {
	raw, err := json.Marshal(record)
	if err != nil {
		return preparation.Record{}, err
	}
	var projected preparation.Record
	err = json.Unmarshal(raw, &projected)
	return projected, err
}

func (OrcaIntentFiles) SamePath(a, b string) bool { return samePath(a, b) }
func (OrcaIntentFiles) ArtifactPaths(record preparation.Record) (string, string) {
	return executionOwnerArtifactPaths(intentFileRecord(record))
}
func (OrcaIntentFiles) ReadToken(record preparation.Record) (string, error) {
	core := intentFileRecord(record)
	return readExecutionLeaseToken(core, claimTokenPath(core))
}
func (OrcaIntentFiles) ReadArtifact(root, path string) ([]byte, error) {
	return readExecutionOwnerArtifact(root, path)
}

func intentFileRecord(record preparation.Record) model.IssueOpsRecord {
	core := model.IssueOpsRecord{ID: record.ID}
	if record.Execution != nil {
		core.Execution = &model.Execution{Workspace: model.Workspace{Root: record.Execution.Workspace.Root}, Lease: model.WriteLease{Generation: record.Execution.Lease.Generation}}
	}
	return core
}
