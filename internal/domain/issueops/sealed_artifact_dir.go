package issueops

import (
	"errors"
	model "issueops/internal/contract/issueops"
	"strings"
)

// ErrSealedArtifactDirMissing는 execution.workspace.artifact_dir 없이 봉인
// 아티팩트에 접근하려 할 때의 거부 사유다. 경로를 추측하지 않고 실패한다.
var ErrSealedArtifactDirMissing = errors.New("execution.workspace.artifact_dir is missing; abandon this cycle and prepare it again from a numbered issue")

// RequireSealedArtifactDir는 레코드가 execution prepare가 고른 봉인 아티팩트
// 디렉터리를 갖고 있는지 확인한다.
func RequireSealedArtifactDir(record model.IssueOpsRecord) error {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.ArtifactDir) == "" {
		return ErrSealedArtifactDirMissing
	}
	return nil
}
