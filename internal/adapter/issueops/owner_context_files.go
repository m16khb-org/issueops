package issueops

import (
	model "issueops/internal/contract/issueops"
	"path/filepath"
)

type OwnerContextFiles struct{ StateRoot string }

func (f OwnerContextFiles) Paths(record model.IssueOpsRecord) model.OwnerPaths {
	packet, prompt := executionOwnerArtifactPaths(record)
	return model.OwnerPaths{Packet: packet, Prompt: prompt, Plan: sealedArtifactPath(record, record.Execution.Workspace.Root, "plan"), Report: executionOwnerVerificationReportPath(record), WorktreeBase: filepath.Dir(record.Execution.Workspace.Root)}
}
func (f OwnerContextFiles) RegularFiles(root string, candidates []string) []string {
	return executionOwnerRegularFiles(root, candidates)
}
func (f OwnerContextFiles) Write(root, path string, data []byte) error {
	return writeExecutionOwnerArtifact(root, path, data)
}
func (f OwnerContextFiles) ReadStaged(id string) (map[string]string, error) {
	return readStagedArtifacts(f.StateRoot, id)
}
func (f OwnerContextFiles) ReadLinkedPlan(record model.IssueOpsRecord) (model.OwnerPlanIdentity, error) {
	return readLinkedPlanIdentity(record)
}
func (f OwnerContextFiles) Materialize(record model.IssueOpsRecord) (map[string]string, error) {
	return materializeStagedArtifacts(f.StateRoot, record)
}
func (f OwnerContextFiles) SamePath(a, b string) bool { return samePath(a, b) }
func (f OwnerContextFiles) CreateOrAdoptToken(record model.IssueOpsRecord) (string, error) {
	return createOrAdoptClaimToken(record)
}
func (f OwnerContextFiles) TokenPath(record model.IssueOpsRecord) string {
	return claimTokenPath(record)
}

func (f OwnerContextFiles) ReadArtifact(root, path string) ([]byte, error) {
	return readExecutionOwnerArtifact(root, path)
}
func (f OwnerContextFiles) ArtifactPath(record model.IssueOpsRecord, name string) string {
	return sealedArtifactPath(record, record.Execution.Workspace.Root, name)
}
