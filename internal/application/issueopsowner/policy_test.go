package issueopsowner

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agentmodelcontract "issueops/internal/contract/agentmodel"
	"issueops/internal/contract/issueops"
)

func fixedRoleModel(host string, role agentmodelcontract.Role, _ string) (string, string, error) {
	return host + "-" + string(role), "high", nil
}

func TestPolicyContextResolvesReviewResearchAndReaderRoles(t *testing.T) {
	var repos []string
	policy, err := PolicyContext(issueops.IssueOpsRecord{Repo: "/source"}, issueops.ExecutionPrepareRequest{OwnerHost: " Claude "}, func(host string, role agentmodelcontract.Role, repo string) (string, string, error) {
		repos = append(repos, repo)
		return fixedRoleModel(host, role, repo)
	})
	if err != nil {
		t.Fatal(err)
	}
	if policy.ReviewerModel != "claude-diff-review" || policy.ResearchModel != "claude-research" || policy.ReaderCheckModel != "claude-reader-check" || policy.ReaderCheckEffort != "high" {
		t.Fatalf("policy = %+v", policy)
	}
	if len(repos) != 3 || repos[0] != "/source" {
		t.Fatalf("repos = %v", repos)
	}
}

func TestPolicyContextFailsOnBrokenSettings(t *testing.T) {
	_, err := PolicyContext(issueops.IssueOpsRecord{}, issueops.ExecutionPrepareRequest{OwnerHost: "codex"}, func(string, agentmodelcontract.Role, string) (string, string, error) {
		return "", "", errors.New("/cfg/agent-models.json: unsupported version 2")
	})
	if err == nil || !strings.Contains(err.Error(), "/cfg/agent-models.json") {
		t.Fatalf("err = %v", err)
	}
	if _, err := PolicyContext(issueops.IssueOpsRecord{}, issueops.ExecutionPrepareRequest{}, nil); err == nil {
		t.Fatal("a missing resolver must fail instead of falling back to defaults")
	}
}

// 이미 봉인된 packet에는 reader_check 필드가 없다. resume은 그 packet을 그대로
// 디코드해야 한다.
func TestLegacyPacketWithoutReaderCheckDecodes(t *testing.T) {
	var packet issueops.OwnerContextPacket
	legacy := `{"schema_version":1,"lifecycle_id":"io-1","owner_host":"claude","owner_model":"claude-sonnet-5-5","reviewer_model":"claude-opus-5-5","research_model":"","commands":{}}`
	if err := json.Unmarshal([]byte(legacy), &packet); err != nil || packet.ReaderCheckModel != "" || packet.ReviewerModel != "claude-opus-5-5" {
		t.Fatalf("legacy packet = %+v, %v", packet, err)
	}
}
