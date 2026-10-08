package issueopsowner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	remote "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
)

type ownerFiles struct {
	events   []string
	plan     string
	linked   model.OwnerPlanIdentity
	writeErr error
	writes   map[string][]byte
	records  []model.IssueOpsRecord
}

func (f *ownerFiles) Paths(model.IssueOpsRecord) model.OwnerPaths {
	return model.OwnerPaths{Packet: "/wt/packet.json", Prompt: "/wt/prompt.md", Plan: "/wt/plan.md", Report: "/wt/report.md", WorktreeBase: "/"}
}
func (f *ownerFiles) RegularFiles(string, []string) []string { return []string{"AGENTS.md"} }
func (f *ownerFiles) Write(_ string, path string, data []byte) error {
	f.events = append(f.events, "write:"+path)
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.writes == nil {
		f.writes = map[string][]byte{}
	}
	f.writes[path] = append([]byte(nil), data...)
	return nil
}
func (f *ownerFiles) ReadStaged(string) (map[string]string, error) {
	f.events = append(f.events, "stage")
	return map[string]string{"plan": f.plan}, nil
}
func (f *ownerFiles) ReadLinkedPlan(model.IssueOpsRecord) (model.OwnerPlanIdentity, error) {
	f.events = append(f.events, "linked")
	if f.linked.Path == "" {
		return model.OwnerPlanIdentity{}, errors.New("missing plan")
	}
	return f.linked, nil
}
func (f *ownerFiles) Materialize(r model.IssueOpsRecord) (map[string]string, error) {
	f.events = append(f.events, "materialize")
	f.records = append(f.records, r)
	f.linked = model.OwnerPlanIdentity{Path: "/wt/plan.md", Digest: digest([]byte(f.plan))}
	return map[string]string{"plan": f.linked.Digest}, nil
}
func (f *ownerFiles) SamePath(a, b string) bool { return a == b }
func (f *ownerFiles) CreateOrAdoptToken(model.IssueOpsRecord) (string, error) {
	f.events = append(f.events, "token")
	return "token-digest", nil
}
func (f *ownerFiles) TokenPath(model.IssueOpsRecord) string { return "/wt/token" }

func ownerFixture(t *testing.T) (Service, *ownerFiles, model.IssueOpsRecord, prep.Snapshot, prep.Intent, prep.IntentReceipt) {
	t.Helper()
	f := &ownerFiles{plan: "# plan\n"}
	body := "AC-01: owner can continue\n## Verification\n```sh\ngo test ./...\n```\n"
	r := model.IssueOpsRecord{SchemaVersion: 1, ID: "io-owner", Repo: "/source", IssueURL: "https://github.com/acme/repo/issues/480", Branch: "feature", Phase: model.IssueOpsPhaseImplement, Execution: &model.Execution{Mode: model.ExecutionModeOrca, Lease: model.WriteLease{Generation: 1}, Workspace: model.Workspace{SourceRoot: "/source", Root: "/wt", Branch: "feature", BaseHead: "abc", Driver: "orca"}}}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := prep.Snapshot{RecordRaw: raw}
	intent := prep.Intent{IssueBodySHA256: digest([]byte(body)), StartedAt: "2026-09-30T00:00:00Z", Workspace: prep.WorkspaceRequest{SourceRoot: "/source", Root: "/wt", Branch: "feature", BaseHead: "abc"}}
	receipt := prep.IntentReceipt{Workspace: &prep.OrcaWorkspaceReceipt{Workspace: prep.WorkspaceReceipt{SourceRoot: "/source", Root: "/wt", Branch: "feature", BaseHead: "abc", Driver: "orca"}, RuntimeID: "runtime", RepoID: "repo", WorktreeID: "wt"}}
	s := Service{Files: f, Template: "Owner {LIFECYCLE_ID}\n{PACKET_SHA256}\n", ReadIssue: func(context.Context, string, remote.ExecutionIssueSnapshotRequest) (remote.ExecutionIssueSnapshot, error) {
		f.events = append(f.events, "remote")
		return remote.ExecutionIssueSnapshot{URL: r.IssueURL, Body: body}, nil
	}, ResolveModel: fixedRoleModel}
	return s, f, r, snapshot, intent, receipt
}

func TestPreparationRequiresPlanBeforeRemoteRead(t *testing.T) {
	s, f, _, snapshot, _, _ := ownerFixture(t)
	f.plan = ""
	_, err := s.ReadPreparationEvidence(context.Background(), snapshot)
	if err == nil || !reflect.DeepEqual(f.events, []string{"stage", "linked"}) {
		t.Fatalf("missing plan must stop before remote/effects: %v %v", err, f.events)
	}
}
func TestPreparationRejectsIssueDriftBeforeToken(t *testing.T) {
	s, f, _, snapshot, intent, receipt := ownerFixture(t)
	intent.IssueBodySHA256 = "changed"
	_, err := s.Prepare(context.Background(), snapshot, prep.Command{}, intent, receipt)
	if err == nil || !strings.Contains(err.Error(), "drifted") || !reflect.DeepEqual(f.events, []string{"remote"}) {
		t.Fatalf("drift must prevent token/artifacts: %v %v", err, f.events)
	}
}
func TestPreparationWritesTokenPlanPacketAndPromptInOrder(t *testing.T) {
	s, f, _, snapshot, intent, receipt := ownerFixture(t)
	out, err := s.Prepare(context.Background(), snapshot, prep.Command{ID: "io-owner", OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}, intent, receipt)
	want := []string{"remote", "token", "stage", "materialize", "linked", "write:/wt/packet.json", "write:/wt/prompt.md"}
	if err != nil || !reflect.DeepEqual(f.events, want) {
		t.Fatalf("prepare: %v events=%v", err, f.events)
	}
	if out.PlanPath != "/wt/plan.md" || out.ClaimTokenSHA256 != "token-digest" || out.ContextPacketSHA256 != digest(f.writes[out.ContextPacketPath]) || out.OwnerPromptSHA256 != digest(f.writes[out.OwnerPromptPath]) {
		t.Fatalf("artifact receipt does not describe writes: %+v", out)
	}
	if len(f.records) != 1 || f.records[0].Execution.Workspace.ArtifactDir != ".issueops/issues/480/artifact" || f.records[0].Execution.Workspace.Root != "/wt" {
		t.Fatalf("workspace lost sealed artifact identity: %+v", f.records)
	}
	var packet model.OwnerContextPacket
	if err := json.Unmarshal(f.writes[out.ContextPacketPath], &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Issue.BodySHA256 != intent.IssueBodySHA256 || packet.ArtifactManifest["plan"] != digest([]byte(f.plan)) {
		t.Fatalf("packet not bound to observed issue and plan: %+v", packet)
	}
}
func TestPacketWriteFailurePreventsPromptWrite(t *testing.T) {
	s, f, _, snapshot, intent, receipt := ownerFixture(t)
	f.writeErr = errors.New("disk unavailable")
	_, err := s.Prepare(context.Background(), snapshot, prep.Command{OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}, intent, receipt)
	if !errors.Is(err, f.writeErr) || f.events[len(f.events)-1] != "write:/wt/packet.json" || len(f.writes) != 0 {
		t.Fatalf("write failure did not stop publication: %v %v", err, f.events)
	}
}
func TestResealDetectsPlanChangeDuringRemoteRead(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.PlanPath = "/wt/plan.md"
	r.Execution.Orca = &model.OrcaBinding{OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}
	f.linked = model.OwnerPlanIdentity{Path: r.PlanPath, Digest: digest([]byte(f.plan))}
	read := s.ReadIssue
	s.ReadIssue = func(ctx context.Context, p string, req remote.ExecutionIssueSnapshotRequest) (remote.ExecutionIssueSnapshot, error) {
		out, err := read(ctx, p, req)
		f.plan = "# changed plan\n"
		f.linked.Digest = digest([]byte(f.plan))
		return out, err
	}
	_, err := s.Reseal(context.Background(), r)
	var required *domain.OwnerPlanRequiredError
	if !errors.As(err, &required) || len(f.writes) != 0 {
		t.Fatalf("reseal accepted plan changed after preflight: %v %v", err, f.events)
	}
}
