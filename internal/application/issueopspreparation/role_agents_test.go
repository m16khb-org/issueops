package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	agentmodelcontract "issueops/internal/contract/agentmodel"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestPrepareOrcaInjectsRoleAgentsIntoTerminalStage(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.models.args = []string{"-c", `agents.issueops-research.config_file="/state/agent-roles/a.toml"`}
	if _, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca)); err != nil {
		t.Fatal(err)
	}
	for _, request := range fixture.gateway.invoked {
		want := []string(nil)
		if request.Stage == preparationcontract.IntentStageTerminal {
			want = fixture.models.args
		}
		if !reflect.DeepEqual(request.RoleAgentArgs, want) {
			t.Fatalf("stage %s role agent args = %q, want %q", request.Stage, request.RoleAgentArgs, want)
		}
	}
	if !reflect.DeepEqual(fixture.models.argsCalls, []string{"codex@/repo"}) {
		t.Fatalf("role agents resolved for %v, want the sealed probe host and repo", fixture.models.argsCalls)
	}
}

func TestPrepareOrcaRoleAgentFailureKeepsNotInvoked(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.models.argsErr = errors.New("/repo/.issueops/agent-models.local.json: unsupported version 2")
	_, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
	if err == nil || !strings.Contains(err.Error(), "agent-models.local.json") {
		t.Fatalf("err = %v", err)
	}
	for _, forbidden := range []string{"mark:terminal_create", "invoke:terminal_create"} {
		if slices.Contains(fixture.trace, forbidden) {
			t.Fatalf("a settings error must stop before %s: %v", forbidden, fixture.trace)
		}
	}
	if countTracePrefix(fixture.trace, "failure:") != 0 {
		t.Fatalf("a settings error is not an Orca failure: %v", fixture.trace)
	}
	if !slices.Contains(fixture.trace, "apply:worktree_create") {
		t.Fatalf("the worktree stage must still complete: %v", fixture.trace)
	}
}

func TestPreparationOwnerDefaultsUseChildImplementForDelegatedRecords(t *testing.T) {
	for name, delegation := range map[string]string{"parent": "", "null": "null", "child": `{"parent_id":"io-parent"}`} {
		fixture := newOrcaApplicationFixture()
		fixture.repository.snapshot.Record.Delegation = []byte(delegation)
		command := orcaCommand(false, preparationcontract.ModeOrca)
		command.OwnerModel, command.OwnerEffort = "", ""
		if _, err := fixture.service.Prepare(context.Background(), command); err != nil {
			t.Fatal(err)
		}
		want := agentmodelcontract.RoleImplement
		if name == "child" {
			want = agentmodelcontract.RoleChildImplement
		}
		if !reflect.DeepEqual(fixture.models.ownerCalls, []agentmodelcontract.Role{want}) {
			t.Fatalf("%s owner roles = %v, want %s", name, fixture.models.ownerCalls, want)
		}
	}
}

func TestPreparationBrokenSettingsFailBeforeAnyStateChange(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.models.ownerErr = errors.New("/xdg/issueops/agent-models.json: json: unknown field \"modle\"")
	command := orcaCommand(false, preparationcontract.ModeOrca)
	command.OwnerEffort = ""
	if _, err := fixture.service.Prepare(context.Background(), command); err == nil || !strings.Contains(err.Error(), "/xdg/issueops/agent-models.json") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(fixture.trace, []string{"load"}) {
		t.Fatalf("a settings error must stop right after load: %v", fixture.trace)
	}

}

// owner 모델과 effort를 둘 다 명시해도 같은 설정 파일이 owner packet의 리뷰·조사·
// 독자 검토 모델을 정한다. 깨진 파일은 Orca worktree를 만들기 전에 드러나야 한다.
func TestPreparationBrokenSettingsFailBeforeOrcaEvenWithExplicitOwnerFlags(t *testing.T) {
	for _, host := range []string{"codex", "omo", "omp"} {
		for _, confirm := range []bool{false, true} {
			fixture := newOrcaApplicationFixture()
			fixture.models.ownerErr = errors.New("/repo/.issueops/agent-models.local.json: unsupported version 2")
			command := orcaCommand(confirm, preparationcontract.ModeOrca)
			command.OwnerHost = host
			_, err := fixture.service.Prepare(context.Background(), command)
			if err == nil || !strings.Contains(err.Error(), "agent-models.local.json") {
				t.Fatalf("%s confirm=%v err = %v", host, confirm, err)
			}
			if !reflect.DeepEqual(fixture.trace, []string{"load"}) {
				t.Fatalf("%s confirm=%v: a settings error must stop right after load: %v", host, confirm, fixture.trace)
			}
		}
	}
}
