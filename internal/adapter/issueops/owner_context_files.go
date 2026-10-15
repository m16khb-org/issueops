package issueops

import (
	model "issueops/internal/contract/issueops"
)

// OwnerContextFiles는 workspace 준비가 쓰는 파일 관측 경계다.
type OwnerContextFiles struct{ StateRoot string }

func (f OwnerContextFiles) Materialize(record model.IssueOpsRecord) (map[string]string, error) {
	return materializeStagedArtifacts(f.StateRoot, record)
}
func (f OwnerContextFiles) SamePath(a, b string) bool { return samePath(a, b) }
