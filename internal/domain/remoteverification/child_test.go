package remoteverification

import (
	"strings"
	"testing"

	model "issueops/internal/contract/remoteverification"
)

func TestChildTargetClassifiesProviders(t *testing.T) {
	gh, err := ChildTarget(" https://github.com/o/r/issues/1 ")
	if err != nil || gh.Provider != "github" || gh.URL != "https://github.com/o/r/issues/1" {
		t.Fatalf("gh=%+v err=%v", gh, err)
	}
	gl, err := ChildTarget("https://gitlab.example.com/grp/sub/proj/-/work_items/7")
	if err != nil || gl.Provider != "gitlab" || gl.Project != "grp/sub/proj" || gl.IID != "7" || gl.Kind != "work_items" {
		t.Fatalf("gl=%+v err=%v", gl, err)
	}
	other, err := ChildTarget("https://example.com/x")
	if err != nil || other.Provider != "" || other.Hostname != "example.com" {
		t.Fatalf("other=%+v err=%v", other, err)
	}
	if _, err := ChildTarget("https://gitlab.example.com/grp/proj"); err == nil || !strings.Contains(err.Error(), "GitLab issue or work item URL") {
		t.Fatalf("err=%v", err)
	}
	if _, err := ChildTarget("http://[bad"); err == nil {
		t.Fatal("unparseable URL accepted")
	}
}

func TestValidateTaskAcceptsEitherTaskTypeCaseInsensitively(t *testing.T) {
	for _, m := range []model.TaskMetadata{{Type: "task"}, {IssueType: "TASK"}} {
		if err := ValidateTask("5", m); err != nil {
			t.Fatalf("%+v: %v", m, err)
		}
	}
	if err := ValidateTask("5", model.TaskMetadata{Empty: true}); err == nil || !strings.Contains(err.Error(), "did not return task metadata") {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateTask("5", model.TaskMetadata{Type: "ISSUE", IssueType: "incident"}); err == nil || !strings.Contains(err.Error(), "did not return a Task work item") {
		t.Fatalf("err=%v", err)
	}
}

func TestRequireMerged(t *testing.T) {
	if err := RequireMerged("u", true); err != nil {
		t.Fatal(err)
	}
	if err := RequireMerged("u", false); err == nil || err.Error() != "remote artifact is not verified merged: u" {
		t.Fatalf("err=%v", err)
	}
}
