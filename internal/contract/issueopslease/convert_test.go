package issueopslease_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
)

// fillNonZero는 모든 field(중첩 struct, pointer, slice 포함)를 0이 아닌 값으로 채운다.
// 새 field가 shape에 추가되면 이 값에도 자동으로 들어가므로, 변환이 그 field를
// 빠뜨리면 아래 JSON 대조가 실패한다.
func fillNonZero(value reflect.Value, path string) {
	switch value.Kind() {
	case reflect.String:
		value.SetString("v:" + path)
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(7)
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
		fillNonZero(value.Elem(), path)
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		fillNonZero(value.Index(0), path+"[0]")
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Type().Field(index)
			fillNonZero(value.Field(index), path+"."+field.Name)
		}
	default:
		panic(fmt.Sprintf("fillNonZero: unsupported kind %s at %s", value.Kind(), path))
	}
}

func TestExecutionToIssueOpsCopiesEveryField(t *testing.T) {
	var source leasecontract.Execution
	fillNonZero(reflect.ValueOf(&source).Elem(), "execution")

	converted := source.ToIssueOps()

	want, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(converted)
	if err != nil {
		t.Fatal(err)
	}
	var wantTree, gotTree any
	if err := json.Unmarshal(want, &wantTree); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &gotTree); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotTree, wantTree) {
		t.Fatalf("conversion dropped or altered a field\n got: %s\nwant: %s", got, want)
	}
	if converted.Workspace.ArtifactDir != source.Workspace.ArtifactDir || converted.Workspace.ArtifactDir == "" {
		t.Fatalf("ArtifactDir dropped: %+v", converted.Workspace)
	}
	if converted.SyncBaseResolution == nil || converted.SyncBaseResolution.Actor.SessionProcess == nil {
		t.Fatalf("sync-base resolution dropped: %+v", converted.SyncBaseResolution)
	}
}

func TestWorkspaceToIssueOpsCopiesEveryField(t *testing.T) {
	var source leasecontract.Workspace
	fillNonZero(reflect.ValueOf(&source).Elem(), "workspace")

	want := issueopscontract.Workspace{
		SourceRoot: source.SourceRoot, Root: source.Root, Branch: source.Branch, BaseHead: source.BaseHead,
		ParentWorktree: source.ParentWorktree, LinkedAt: source.LinkedAt, ArtifactDir: source.ArtifactDir,
	}
	if got := source.ToIssueOps(); got != want || got.ArtifactDir == "" {
		t.Fatalf("workspace conversion = %+v, want %+v", got, want)
	}
}

func TestExecutionToIssueOpsKeepsNilAndEmptyDistinct(t *testing.T) {
	bare := leasecontract.Execution{Lease: leasecontract.Lease{Generation: 2, Status: "released"}}.ToIssueOps()
	if bare.Lease.Holder != nil || bare.Lease.Generation != 2 || bare.Lease.Status != issueopscontract.LeaseStatusReleased ||
		bare.Pending != nil || bare.Completion != nil || bare.CompletionHistory != nil || bare.Failure != nil ||
		bare.SyncBaseResolution != nil || bare.SyncBaseEvents != nil {
		t.Fatalf("bare execution materialized optional fields: %+v", bare)
	}

	// `verification`은 omitempty가 아니므로 nil은 null, 빈 slice는 []로 나간다.
	empty := leasecontract.Execution{Completion: &leasecontract.Completion{Verification: []string{}}}.ToIssueOps()
	if empty.Completion.Verification == nil {
		t.Fatal("empty verification must stay an empty slice")
	}
	nilVerification := leasecontract.Execution{Completion: &leasecontract.Completion{}}.ToIssueOps()
	if nilVerification.Completion.Verification != nil {
		t.Fatalf("nil verification must stay nil: %#v", nilVerification.Completion.Verification)
	}

	noProcess := leasecontract.Execution{Lease: leasecontract.Lease{Holder: &leasecontract.Actor{Host: "codex", SessionID: "s"}}}.ToIssueOps()
	if noProcess.Lease.Holder == nil || noProcess.Lease.Holder.SessionProcess != nil {
		t.Fatalf("nil session process must stay nil: %+v", noProcess.Lease.Holder)
	}
}

func TestExecutionToIssueOpsDoesNotAliasSource(t *testing.T) {
	var source leasecontract.Execution
	fillNonZero(reflect.ValueOf(&source).Elem(), "execution")
	converted := source.ToIssueOps()

	converted.Completion.Verification[0] = "changed"
	converted.CompletionHistory[0].Completion.Verification[0] = "changed"
	converted.SyncBaseResolution.ConflictFiles[0] = "changed"
	converted.Lease.Holder.SessionProcess.PID = 99
	converted.Pending.Kind = "changed"

	if source.Completion.Verification[0] == "changed" || source.CompletionHistory[0].Completion.Verification[0] == "changed" ||
		source.SyncBaseResolution.ConflictFiles[0] == "changed" || source.Lease.Holder.SessionProcess.PID == 99 ||
		source.Pending.Kind == "changed" {
		t.Fatalf("converted execution aliases its source: %+v", source)
	}
}
