package issueops

import (
	model "issueops/internal/contract/issueops"
)

// OwnerContextFiles 타입은 workspace 준비가 쓰는 파일 관측 경계다.
type OwnerContextFiles struct{ StateRoot string }

// Materialize 메서드는 스테이징된 artifact를 record의 워크트리 봉인 디렉터리에 쓰고
// name→sha256 manifest를 돌려준다. 이미 있는 파일을 바꾸는 재실행은 거부된다.
func (f OwnerContextFiles) Materialize(record model.Record) (map[string]string, error) {
	return materializeStagedArtifacts(f.StateRoot, record)
}

// SamePath 메서드는 a와 b가 같은 위치인지 보고한다. 존재하는 부분의 symlink는 풀어서 비교한다.
func (f OwnerContextFiles) SamePath(a, b string) bool { return samePath(a, b) }
