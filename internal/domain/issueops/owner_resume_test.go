package issueops

import (
	"crypto/sha256"
	"fmt"
	model "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

func TestOwnerResumePacketIdentityAndBody(t *testing.T) {
	body := "AC-01: sealed issue"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	record := model.IssueOpsRecord{ID: "io-resume", IssueURL: "https://github.com/acme/repo/issues/1", Execution: &model.Execution{Mode: model.ExecutionModeOrca, Lease: model.WriteLease{Generation: 3}, Workspace: model.Workspace{Branch: "branch", BaseHead: "head"}}}
	baseline := model.OwnerContextPacket{SchemaVersion: model.IssueOpsSchemaVersion, LifecycleID: record.ID, Mode: record.Execution.Mode, LeaseGeneration: 3, Branch: "branch", BaseHead: "head", Issue: model.OwnerIssue{URL: record.IssueURL, Body: body, BodySHA256: hash}}
	if err := ValidateOwnerResumePacket(record, baseline, hash, true, true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*model.OwnerContextPacket)
		want   string
	}{
		{"schema", func(p *model.OwnerContextPacket) { p.SchemaVersion = 0 }, "execution identity mismatch"},
		{"generation", func(p *model.OwnerContextPacket) { p.LeaseGeneration = 2 }, "execution identity mismatch"},
		{"branch", func(p *model.OwnerContextPacket) { p.Branch = "other" }, "execution identity mismatch"},
		{"base", func(p *model.OwnerContextPacket) { p.BaseHead = "other" }, "execution identity mismatch"},
		{"issue url", func(p *model.OwnerContextPacket) { p.Issue.URL = "other" }, "execution identity mismatch"},
		{"declared digest", func(p *model.OwnerContextPacket) { p.Issue.BodySHA256 = strings.Repeat("f", 64) }, "issue body digest mismatch"},
		{"body", func(p *model.OwnerContextPacket) { p.Issue.Body = "forged" }, "issue body does not hash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet := baseline
			tc.change(&packet)
			err := ValidateOwnerResumePacket(record, packet, hash, true, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
	for _, paths := range [][2]bool{{false, true}, {true, false}} {
		if err := ValidateOwnerResumePacket(record, baseline, hash, paths[0], paths[1]); err == nil {
			t.Fatal("mismatched canonical path accepted")
		}
	}
}
func TestOwnerResumeTokenBounds(t *testing.T) {
	for _, tc := range []struct {
		body, want string
		ok         bool
	}{{" token\n", "token", true}, {strings.Repeat("a", 256), strings.Repeat("a", 256), true}, {strings.Repeat("a", 257), "oversized", false}, {" \n", "empty", false}} {
		token, err := ParseOwnerResumeToken([]byte(tc.body))
		if tc.ok {
			if err != nil || token != tc.want {
				t.Fatalf("valid token: %d %v", len(token), err)
			}
		} else if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("invalid token: %v", err)
		}
	}
}
