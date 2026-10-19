package issueops

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	model "issueops/internal/contract/issueops"
)

type ReviewPlanSource struct{}

func (ReviewPlanSource) LinkedDigest(record model.IssueOpsRecord) (string, error) {
	identity, err := readLinkedPlanIdentity(record)
	if err != nil {
		return "", err
	}
	return identity.Digest, nil
}

func (ReviewPlanSource) StagedPlans(stateRoot, id string) (map[string]string, error) {
	return readStagedArtifacts(stateRoot, id)
}

// ReviewArchitectureSource는 architecture review가 묶이는 문서를 읽는다.
type ReviewArchitectureSource struct{}

// Document는 실행 전에는 스테이징한 architecture를, 실행 뒤에는 봉인 파일을 읽는다.
// 문서가 없으면 빈 문자열과 nil 오류를 돌려주고, 그 판정은 domain이 한다.
func (ReviewArchitectureSource) Document(stateRoot string, record model.IssueOpsRecord) (string, error) {
	if record.Execution == nil {
		staged, err := readStagedArtifacts(stateRoot, record.ID)
		if err != nil {
			return "", err
		}
		return staged["architecture"], nil
	}
	if sealedArtifactDir(record) == "" {
		return "", nil
	}
	content, err := os.ReadFile(sealedArtifactPath(record, record.Execution.Workspace.Root, "architecture"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(content), err
}

// SealedDigest는 봉인된 architecture 파일의 sha256이다. 파일이 없거나 비면 오류다.
func (source ReviewArchitectureSource) SealedDigest(record model.IssueOpsRecord) (string, error) {
	if record.Execution == nil {
		return "", errors.New("architecture is sealed only after execution prepare")
	}
	body, err := source.Document("", record)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) == "" {
		return "", errors.New("sealed architecture is missing or empty")
	}
	return digestBytes([]byte(body)), nil
}
