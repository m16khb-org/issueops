package issueops

import (
	"os"

	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
)

type CompletionArtifacts struct{}

func (CompletionArtifacts) Read(record model.IssueOpsRecord, root, name string) (string, bool) {
	return readCompletionArtifact(sealedArtifactPath(record, root, name))
}

func readCompletionArtifact(path string) (string, bool) {
	info, err := os.Lstat(path)
	// 형제 리더(execution_owner_context.go)와 동일한 봉인 계약: 0600 정규
	// 파일만 공개면 게시 대상이다. staging을 우회해 이 디렉토리에 놓인 임의
	// 파일이 이슈 본문으로 퍼블리시되는 경로를 차단한다(C3-F3).
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > leasecontract.OwnerArtifactMaxBytes {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}
