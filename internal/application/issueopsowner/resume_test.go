package issueopsowner

import (
	"encoding/json"
	"errors"
	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	"reflect"
	"strings"
	"testing"
)

type resumeFiles struct {
	*ownerFiles
	data        map[string][]byte
	packetReads int
	pathChecks  int
	swapPacket  bool
}

func (f *resumeFiles) ReadArtifact(_ string, path string) ([]byte, error) {
	f.events = append(f.events, "read:"+path)
	if path == "/wt/packet.json" {
		f.packetReads++
		if f.swapPacket && f.packetReads == 2 {
			return []byte("{}"), nil
		}
	}
	data, ok := f.data[path]
	if !ok {
		return nil, errors.New("missing artifact")
	}
	return data, nil
}
func (f *resumeFiles) ArtifactPath(_ model.IssueOpsRecord, name string) string {
	return "/wt/" + name + ".md"
}
func resumeFixture(t *testing.T) (ResumeReader, *resumeFiles, model.IssueOpsRecord) {
	t.Helper()
	s, files, r, snapshot, intent, receipt := ownerFixture(t)
	out, err := s.Prepare(t.Context(), snapshot, prep.Command{OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}, intent, receipt)
	if err != nil {
		t.Fatal(err)
	}
	f := &resumeFiles{ownerFiles: files, data: files.writes}
	f.data["/wt/token"] = []byte("token\n")
	f.data["/wt/plan.md"] = []byte(f.plan)
	r.PlanPath = out.PlanPath
	r.Execution.Workspace.ArtifactDir = ".issueops/issues/199/artifact"
	r.Execution.Lease.ClaimTokenSHA256 = digest([]byte("token"))
	r.Execution.Orca = &model.OrcaBinding{ArtifactIdentityVersion: model.OrcaArtifactIdentityVersion, IssueBodySHA256: intent.IssueBodySHA256, ContextPacketSHA256: out.ContextPacketSHA256, OwnerPromptSHA256: out.OwnerPromptSHA256, OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}
	f.events = nil
	return ResumeReader{Files: f}, f, r
}
func TestResumeReadsImmutableArtifactsInOrder(t *testing.T) {
	reader, f, r := resumeFixture(t)
	out, err := reader.Read(r)
	want := []string{"read:/wt/token", "read:/wt/packet.json", "read:/wt/packet.json", "read:/wt/plan.md", "linked", "read:/wt/prompt.md"}
	if err != nil || !reflect.DeepEqual(f.events, want) {
		t.Fatalf("resume order: %v %v", err, f.events)
	}
	if out.ContextPacketSHA256 != r.Execution.Orca.ContextPacketSHA256 || out.ClaimTokenPath != "/wt/token" {
		t.Fatalf("resume receipt: %+v", out)
	}
}
func TestResumeRejectsPacketChangedBetweenReads(t *testing.T) {
	reader, f, r := resumeFixture(t)
	f.swapPacket = true
	_, err := reader.Read(r)
	if err == nil || !strings.Contains(err.Error(), "sealed context packet digest mismatch") || f.packetReads != 2 || len(f.events) != 3 {
		t.Fatalf("second read failed to detect drift before plan/prompt: %v %v", err, f.events)
	}
}
func TestResumeRejectsBodyTamperEvenWhenPacketDigestMatches(t *testing.T) {
	reader, f, r := resumeFixture(t)
	var packet model.OwnerContextPacket
	if err := json.Unmarshal(f.data["/wt/packet.json"], &packet); err != nil {
		t.Fatal(err)
	}
	packet.Issue.Body = "changed body"
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	f.data["/wt/packet.json"] = data
	r.Execution.Orca.ContextPacketSHA256 = digest(data)
	_, err = reader.Read(r)
	if err == nil || !strings.Contains(err.Error(), "issue body does not hash") {
		t.Fatalf("accepted forged packet body: %v", err)
	}
}
func TestResumeTokenFailurePrecedesMissingBinding(t *testing.T) {
	reader, f, r := resumeFixture(t)
	r.Execution.Orca = nil
	f.data["/wt/token"] = []byte("wrong")
	_, err := reader.Read(r)
	if err == nil || err.Error() != "current generation claim token identity changed" || len(f.events) != 1 {
		t.Fatalf("token failure precedence: %v %v", err, f.events)
	}
}

func (f *resumeFiles) SamePath(a, b string) bool { f.pathChecks++; return a == b }
func TestResumeRejectsWrongLifecycleBeforeInspectingPacketPaths(t *testing.T) {
	reader, f, r := resumeFixture(t)
	var packet model.OwnerContextPacket
	if err := json.Unmarshal(f.data["/wt/packet.json"], &packet); err != nil {
		t.Fatal(err)
	}
	packet.LifecycleID = "other"
	packet.SourceRoot = "/untrusted"
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	f.data["/wt/packet.json"] = data
	r.Execution.Orca.ContextPacketSHA256 = digest(data)
	_, err = reader.Read(r)
	if err == nil || !strings.Contains(err.Error(), "execution identity mismatch") || f.pathChecks != 1 {
		t.Fatalf("wrong lifecycle caused packet path observation: %v checks=%d", err, f.pathChecks)
	}
}
