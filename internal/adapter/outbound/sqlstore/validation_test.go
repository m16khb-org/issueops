package sqlstore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/port"
	stateport "issueops/internal/port/state"
)

func TestRawCASErrorReportsRowAndFailedBucket(t *testing.T) {
	err := error(&RawCASError{Bucket: "resume", ID: "record"})
	if got := err.Error(); !strings.Contains(got, "resume/record") {
		t.Fatalf("Error() = %q", got)
	}
	var bucketed interface{ FailedBucket() string }
	if !errors.As(err, &bucketed) || bucketed.FailedBucket() != "resume" {
		t.Fatalf("FailedBucket not exposed: %v", err)
	}
}

func TestNestedSpanErrorNamesActiveAndRequestedRoots(t *testing.T) {
	msg := (&NestedSpanError{ActiveDirs: []string{"/a", "/b"}, RequestedDir: "/b"}).Error()
	if !strings.Contains(msg, `"/b"`) || !strings.Contains(msg, "/a") {
		t.Fatalf("Error() = %q", msg)
	}
}

func TestMutateTranslatesStateMutationsIncludingRequireAbsent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Mutate(ctx, []stateport.Mutation{
		{Bucket: "b", ID: "keep", Data: []byte("v1"), RequireAbsent: true},
		{Bucket: "b", ID: "drop", Data: []byte("x")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Mutate(ctx, []stateport.Mutation{
		{Bucket: "b", ID: "drop", Delete: true},
		{Bucket: "b", ID: "keep", Data: []byte("v2"), RequireAbsent: true},
	}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("RequireAbsent on existing row: %v", err)
	}
	if got, _, _ := db.Get("b", "keep"); string(got) != "v1" {
		t.Fatalf("keep = %q, want rolled-back v1", got)
	}
	if _, ok, _ := db.Get("b", "drop"); !ok {
		t.Fatal("delete in failed batch must roll back")
	}
	if err := db.Mutate(ctx, []stateport.Mutation{{Bucket: "b", ID: "drop", Delete: true}, {Bucket: "b", ID: "new", Data: []byte("n")}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.Get("b", "drop"); ok {
		t.Fatal("drop should be deleted")
	}
	if got, ok, _ := db.Get("b", "new"); !ok || string(got) != "n" {
		t.Fatalf("new = %q ok=%v", got, ok)
	}
}

func TestApplyRejectsInvalidMutationsWithoutWriting(t *testing.T) {
	db := openTestDB(t)
	good := port.RecordMutation{Bucket: "b", ID: "ok", Data: []byte("x")}
	cases := map[string]struct {
		bad  port.RecordMutation
		want string
	}{
		"delete requiring absent": {port.RecordMutation{Bucket: "b", ID: "x", Delete: true, RequireAbsent: true}, "cannot require an absent row"},
		"empty bucket":            {port.RecordMutation{ID: "x", Data: []byte("x")}, "bucket and id are required"},
		"empty id":                {port.RecordMutation{Bucket: "b", Data: []byte("x")}, "bucket and id are required"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := db.Apply(t.Context(), []port.RecordMutation{good, tc.bad}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Apply err = %v", err)
			}
			if _, ok, _ := db.Get("b", "ok"); ok {
				t.Fatal("valid sibling mutation was written despite rejected batch")
			}
		})
	}
	if err := db.Apply(t.Context(), nil); err != nil {
		t.Fatalf("empty batch: %v", err)
	}
}

func TestCompareAndApplyValidatesArguments(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	if err := db.Apply(ctx, []port.RecordMutation{{Bucket: "r", ID: "a", Data: []byte("before")}}); err != nil {
		t.Fatal(err)
	}
	valid := []port.ExpectedRecord{{Bucket: "r", ID: "a", Data: []byte("before")}}
	mut := []port.RecordMutation{{Bucket: "r", ID: "a", Data: []byte("after")}}

	for name, expected := range map[string][]port.ExpectedRecord{
		"empty bucket": {{ID: "a", Data: []byte("before")}},
		"empty id":     {{Bucket: "r", Data: []byte("before")}},
		"nil data":     {{Bucket: "r", ID: "a"}},
	} {
		if err := db.CompareAndApply(ctx, expected, mut); err == nil || !strings.Contains(err.Error(), "expected record bucket, id, and data are required") {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if err := db.CompareAndApply(ctx, valid, []port.RecordMutation{{Bucket: "r", Data: []byte("x")}}); err == nil || !strings.Contains(err.Error(), "bucket and id are required") {
		t.Fatalf("invalid mutation: %v", err)
	}
	if err := db.CompareAndApply(ctx, []port.ExpectedRecord{{Bucket: "r", ID: "a", Data: []byte("stale")}}, nil); err != nil {
		t.Fatalf("empty mutation list must short-circuit before comparing: %v", err)
	}
	if got, _, _ := db.Get("r", "a"); string(got) != "before" {
		t.Fatalf("row = %q", got)
	}
}

func TestCompareAndApplyReportsMissingExpectedRowAsRawCAS(t *testing.T) {
	db := openTestDB(t)
	err := db.CompareAndApply(t.Context(), []port.ExpectedRecord{{Bucket: "r", ID: "ghost", Data: []byte("x")}}, []port.RecordMutation{{Bucket: "r", ID: "ghost", Data: []byte("y")}})
	var stale *RawCASError
	if !errors.As(err, &stale) || stale.ID != "ghost" {
		t.Fatalf("err = %v", err)
	}
	if _, ok, _ := db.Get("r", "ghost"); ok {
		t.Fatal("row must not be created by failed CAS")
	}
}

func TestCompareAndApplyFuncBuilderContract(t *testing.T) {
	db := openTestDB(t)
	ctx := t.Context()
	if err := db.Apply(ctx, []port.RecordMutation{{Bucket: "r", ID: "a", Data: []byte("before")}}); err != nil {
		t.Fatal(err)
	}
	expected := []port.ExpectedRecord{{Bucket: "r", ID: "a", Data: []byte("before")}}

	if err := db.CompareAndApplyFunc(ctx, expected, nil); err == nil || !strings.Contains(err.Error(), "builder is required") {
		t.Fatalf("nil builder: %v", err)
	}
	boom := errors.New("encode boom")
	if err := db.CompareAndApplyFunc(ctx, expected, func() ([]port.RecordMutation, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatalf("builder error: %v", err)
	}
	if err := db.CompareAndApplyFunc(ctx, expected, func() ([]port.RecordMutation, error) { return nil, nil }); err != nil {
		t.Fatalf("empty builder result: %v", err)
	}
	if err := db.CompareAndApplyFunc(ctx, expected, func() ([]port.RecordMutation, error) {
		return []port.RecordMutation{{Bucket: "r", Data: []byte("x")}}, nil
	}); err == nil || !strings.Contains(err.Error(), "bucket and id are required") {
		t.Fatalf("invalid built mutation: %v", err)
	}
	if err := db.CompareAndApplyFunc(ctx, expected, func() ([]port.RecordMutation, error) {
		return []port.RecordMutation{{Bucket: "r", ID: "a", Data: []byte("new")}, {Bucket: "r", ID: "b", Data: []byte("x"), RequireAbsent: true}, {Bucket: "r", ID: "a", Data: []byte("dup"), RequireAbsent: true}}, nil
	}); err == nil || !strings.Contains(err.Error(), "precondition failed") {
		t.Fatalf("require-absent on existing row: %v", err)
	}
	if got, _, _ := db.Get("r", "a"); string(got) != "before" {
		t.Fatalf("row = %q, failed builder paths must leave data untouched", got)
	}
	if _, ok, _ := db.Get("r", "b"); ok {
		t.Fatal("earlier mutation in failed batch leaked")
	}
}

func TestDeleteBeforeRequiresBucketAndID(t *testing.T) {
	db := openTestDB(t)
	if err := db.Apply(t.Context(), []port.RecordMutation{{Bucket: "b", ID: "1", Data: []byte("x")}}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][2]string{{"", "2"}, {"b", ""}} {
		if err := db.DeleteBefore(t.Context(), args[0], args[1]); err == nil || !strings.Contains(err.Error(), "bucket and id are required") {
			t.Fatalf("DeleteBefore(%q,%q) = %v", args[0], args[1], err)
		}
	}
	if _, ok, _ := db.Get("b", "1"); !ok {
		t.Fatal("rejected DeleteBefore removed a row")
	}
}

func TestWithSpanObserverIgnoresNilArguments(t *testing.T) {
	ctx := t.Context()
	if got := WithSpanObserver(ctx, nil); got != ctx {
		t.Fatal("nil observer must return the original context")
	}
	//nolint:staticcheck // nil context is the branch under test
	if got := WithSpanObserver(nil, func(SpanObservation) {}); got != nil {
		t.Fatal("nil context must stay nil")
	}
	if spanObserver(nil) != nil { //nolint:staticcheck
		t.Fatal("nil context has no observer")
	}
	if spanObserver(ctx) != nil {
		t.Fatal("context without observer must yield nil")
	}
	called := false
	observed := WithSpanObserver(ctx, func(SpanObservation) { called = true })
	spanObserver(observed)(SpanObservation{})
	if !called {
		t.Fatal("registered observer not retrieved")
	}
}

func TestListExistingAndGetAllExistingReadEmptyBucketAsEmpty(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("other", "x", []byte("1")); err != nil {
		t.Fatal(err)
	}
	ids, err := ListExisting(dir, "empty")
	if err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("ListExisting = %#v err=%v", ids, err)
	}
	rows, err := GetAllExisting(dir, "empty")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("GetAllExisting = %#v err=%v", rows, err)
	}
	if ids, err := db.List("empty"); err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("List = %#v err=%v", ids, err)
	}
	if rows, err := db.GetAll("empty"); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("GetAll = %#v err=%v", rows, err)
	}
}
