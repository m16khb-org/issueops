package issueopsrecord

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
)

func putRecord(t *testing.T, root string, record issueopscontract.IssueOpsRecord) {
	t.Helper()
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(bucket, record.ID, data); err != nil {
		t.Fatal(err)
	}
}

func baseRecord(id string) issueopscontract.IssueOpsRecord {
	return issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Repo:          "/repo",
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	}
}

func TestStoreRejectsCancelledContextBeforeTouchingState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	store := Store{}
	mutate := func(r issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		t.Error("mutation ran under cancelled context")
		return r, false, nil
	}
	related := func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
		t.Error("related mutation ran under cancelled context")
		return nil, false, nil
	}
	calls := map[string]func() error{
		"read": func() error { _, err := store.Read(ctx, root, "io-x"); return err },
		"list": func() error { _, err := store.ListIDs(ctx, root); return err },
		"scan": func() error { _, _, err := store.Scan(ctx, root); return err },
		"update": func() error {
			_, err := store.Update(ctx, root, "io-x", mutate)
			return err
		},
		"update related": func() error {
			_, err := store.UpdateRelated(ctx, root, "io-x", "b", related)
			return err
		},
		"read related": func() error { _, _, err := store.ReadRelated(ctx, root, "io-x", "b"); return err },
		"delete":       func() error { return store.Delete(ctx, root, "io-x") },
		"delete if unchanged": func() error {
			return store.DeleteIfUnchanged(ctx, root, "io-x", baseRecord("io-x"))
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
	if _, err := sqlstore.ListExisting(root, bucket); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cancelled calls created state: %v", err)
	}
}

func TestStoreRejectsInvalidIDsBeforeTouchingState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	store := Store{}
	ctx := t.Context()
	mutate := func(r issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		t.Error("mutation ran for invalid id")
		return r, false, nil
	}
	related := func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
		t.Error("related mutation ran for invalid id")
		return nil, false, nil
	}
	for _, tc := range []struct{ id, want string }{
		{"   ", "id is required"},
		{"issue-1", `invalid issueops id "issue-1": `+issueopscontract.IDFormatHint},
		{"io-a/b", `invalid issueops id "io-a/b"`},
		{"io-..x", `invalid issueops id "io-..x"`},
	} {
		calls := map[string]func() error{
			"read":    func() error { _, err := store.Read(ctx, root, tc.id); return err },
			"update":  func() error { _, err := store.Update(ctx, root, tc.id, mutate); return err },
			"related": func() error { _, err := store.UpdateRelated(ctx, root, tc.id, "b", related); return err },
			"read related": func() error {
				_, _, err := store.ReadRelated(ctx, root, tc.id, "b")
				return err
			},
			"delete": func() error { return store.Delete(ctx, root, tc.id) },
			"delete if unchanged": func() error {
				return store.DeleteIfUnchanged(ctx, root, tc.id, baseRecord("io-ok"))
			},
		}
		for name, call := range calls {
			if err := call(); err == nil || err.Error() != tc.want {
				t.Errorf("%s(%q) error = %v, want %q", name, tc.id, err, tc.want)
			}
		}
	}
	if _, err := sqlstore.ListExisting(root, bucket); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("invalid ids created state: %v", err)
	}
}

func TestStoreListIDsReturnsSortedIDsAndEmptyWithoutStore(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	ids, err := (Store{}).ListIDs(t.Context(), root)
	if err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("missing store ids = %#v, err = %v", ids, err)
	}
	putRecord(t, root, baseRecord("io-b"))
	putRecord(t, root, baseRecord("io-a"))
	ids, err = (Store{}).ListIDs(t.Context(), root)
	if err != nil || len(ids) != 2 || ids[0] != "io-a" || ids[1] != "io-b" {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}

func TestStoreScanWithoutStoreReturnsEmptyInventory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	records, diagnostics, err := (Store{}).Scan(t.Context(), root)
	if err != nil || len(records) != 0 || len(diagnostics) != 0 || records == nil || diagnostics == nil {
		t.Fatalf("records=%#v diagnostics=%#v err=%v", records, diagnostics, err)
	}
}

func TestStoreReadReportsMissingRecordAsNotExist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	putRecord(t, root, baseRecord("io-present"))
	record, err := (Store{}).Read(t.Context(), root, "io-absent")
	if !errors.Is(err, fs.ErrNotExist) || record.OK || record.ID != "io-absent" {
		t.Fatalf("record=%+v err=%v", record, err)
	}
	if !strings.Contains(err.Error(), "issueops record io-absent") {
		t.Fatalf("error lacks record id: %v", err)
	}
}

func TestStoreUpdateMissingRecordAndMutationOutcomes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	store := Store{}
	putRecord(t, root, baseRecord("io-upd"))

	_, err := store.Update(t.Context(), root, "io-missing", func(r issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		t.Error("mutation ran for missing record")
		return r, true, nil
	})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing update error = %v", err)
	}

	boom := errors.New("boom")
	result, err := store.Update(t.Context(), root, "io-upd", func(r issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		r.Branch = "discarded"
		return r, true, boom
	})
	if !errors.Is(err, boom) || result.OK {
		t.Fatalf("mutation error result=%+v err=%v", result, err)
	}

	result, err = store.Update(t.Context(), root, "io-upd", func(r issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		r.Branch = "unsaved"
		return r, false, nil
	})
	if err != nil || !result.OK {
		t.Fatalf("no-change result=%+v err=%v", result, err)
	}
	stored, err := store.Read(t.Context(), root, "io-upd")
	if err != nil || stored.Branch != "" {
		t.Fatalf("mutation with changed=false or error persisted: %+v err=%v", stored, err)
	}
}

func TestStoreRefusesRecordsWithApplyingAbandonFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	record := baseRecord("io-abandon")
	record.Phase = issueopscontract.IssueOpsPhaseDone
	record.CleanupAbandonFailure = &issueopscontract.IssueOpsCleanupAbandonFailure{
		Step: issueopscontract.CleanupFailureStepApplying,
	}
	putRecord(t, root, record)

	called := false
	_, err := (Store{}).Update(t.Context(), root, record.ID, func(r issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
		called = true
		return r, true, nil
	})
	if err == nil || err.Error() != "cleanup abandon apply is in progress" || called {
		t.Fatalf("update err=%v called=%v", err, called)
	}
	_, err = (Store{}).UpdateRelated(t.Context(), root, record.ID, "rel", func(_ issueopscontract.IssueOpsRecord, _ []byte, _ bool) ([]byte, bool, error) {
		called = true
		return []byte("x"), false, nil
	})
	if err == nil || err.Error() != "cleanup abandon apply is in progress" || called {
		t.Fatalf("related update err=%v called=%v", err, called)
	}
}

func TestStoreUpdateRelatedCreatesReplacesAndRemovesRelatedRow(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	store := Store{}
	putRecord(t, root, baseRecord("io-rel"))
	type seen struct {
		data  string
		found bool
	}
	var observed []seen
	step := func(next []byte, remove bool) {
		t.Helper()
		result, err := store.UpdateRelated(t.Context(), root, "io-rel", "rel_v1", func(r issueopscontract.IssueOpsRecord, data []byte, found bool) ([]byte, bool, error) {
			observed = append(observed, seen{string(data), found})
			if r.ID != "io-rel" {
				t.Errorf("mutation saw record %q", r.ID)
			}
			return next, remove, nil
		})
		if err != nil || !result.OK || result.ID != "io-rel" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if result.Repo != "/repo" || result.Phase != issueopscontract.IssueOpsPhaseProblem {
			t.Fatalf("result must carry the record read inside the span: %+v", result)
		}
	}
	step([]byte("one"), false)
	step([]byte("two"), false)
	step(nil, true)
	step([]byte("three"), false)

	want := []seen{{"", false}, {"one", true}, {"two", true}, {"", false}}
	for i := range want {
		if observed[i] != want[i] {
			t.Errorf("step %d saw %+v, want %+v", i, observed[i], want[i])
		}
	}
	data, found, err := store.ReadRelated(t.Context(), root, "io-rel", "rel_v1")
	if err != nil || !found || string(data) != "three" {
		t.Fatalf("related = %q found=%v err=%v", data, found, err)
	}

	boom := errors.New("boom")
	_, err = store.UpdateRelated(t.Context(), root, "io-rel", "rel_v1", func(issueopscontract.IssueOpsRecord, []byte, bool) ([]byte, bool, error) {
		return []byte("lost"), false, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v", err)
	}
	data, _, _ = store.ReadRelated(t.Context(), root, "io-rel", "rel_v1")
	if string(data) != "three" {
		t.Fatalf("failed mutation changed related row: %q", data)
	}

	_, err = store.UpdateRelated(t.Context(), root, "io-none", "rel_v1", func(issueopscontract.IssueOpsRecord, []byte, bool) ([]byte, bool, error) {
		t.Error("mutation ran for missing record")
		return nil, false, nil
	})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing record error = %v", err)
	}
}

func TestStoreDeleteOfMissingRecordIsNoOpAndKeepsRelatedRows(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put("rel_v1", "io-ghost", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	if err := (Store{}).Delete(t.Context(), root, "io-ghost", "rel_v1"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := database.Get("rel_v1", "io-ghost"); err != nil || !found {
		t.Fatalf("related row removed without a record: found=%v err=%v", found, err)
	}
}

func TestDeleteIfUnchangedRejectsInvalidExpectedRecord(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	putRecord(t, root, baseRecord("io-inv"))
	expected := baseRecord("io-inv")
	expected.Phase = "unknown"
	if err := (Store{}).DeleteIfUnchanged(t.Context(), root, "io-inv", expected); err == nil || err.Error() != "invalid state" {
		t.Fatalf("error = %v", err)
	}
	if _, err := (Store{}).Read(t.Context(), root, "io-inv"); err != nil {
		t.Fatalf("record deleted despite invalid expected: %v", err)
	}
}

func TestEncodeRejectsInvalidID(t *testing.T) {
	record := baseRecord("not-an-id")
	if _, err := Encode(record); err == nil || err.Error() != `invalid issueops id "not-an-id": `+issueopscontract.IDFormatHint {
		t.Fatalf("error = %v", err)
	}
}

func TestNormalizeIDTrimsWhitespaceAndRejectsBackslash(t *testing.T) {
	id, err := NormalizeID("  io-padded \n")
	if err != nil || id != "io-padded" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if _, err := NormalizeID(`io-a\b`); err == nil {
		t.Fatal("backslash id accepted")
	}
}

func TestEncodeLeaseRejectsWrongSchemaVersion(t *testing.T) {
	_, err := EncodeLease(leasecontract.Record{SchemaVersion: leasecontract.SchemaVersion + 1, ID: "io-lease"})
	if err == nil || err.Error() != "invalid state" {
		t.Fatalf("error = %v", err)
	}
}

func TestJSONLineObserverDefaultsThresholdAndIgnoresNilWriter(t *testing.T) {
	NewJSONLineObserver(nil, time.Second).Observe(SpanObservation{Outcome: "error"})

	var output bytes.Buffer
	observer := NewJSONLineObserver(&output, 0)
	observer.Observe(SpanObservation{Outcome: sqlstore.SpanOutcomeSuccess, WaitMS: defaultSlowSpanThreshold.Milliseconds() - 1})
	if output.Len() != 0 {
		t.Fatalf("below default threshold emitted: %q", output.String())
	}
	observer.Observe(SpanObservation{Outcome: sqlstore.SpanOutcomeSuccess, HoldMS: defaultSlowSpanThreshold.Milliseconds()})
	if !strings.Contains(output.String(), `"event":"issueops_record_span"`) {
		t.Fatalf("at default threshold not emitted: %q", output.String())
	}
}

func TestJSONLineObserverEmitsFailedSpansAndKeepsProvidedTimestamp(t *testing.T) {
	var output bytes.Buffer
	NewJSONLineObserver(&output, time.Hour).Observe(SpanObservation{
		Outcome:     "error",
		GeneratedAt: "2026-01-02T03:04:05Z",
	})
	line := output.String()
	if !strings.Contains(line, `"generated_at":"2026-01-02T03:04:05Z"`) || !strings.HasSuffix(line, "\n") {
		t.Fatalf("line = %q", line)
	}
}

func TestJSONLineObserverLeavesFastCommandErrorsToTheCaller(t *testing.T) {
	var output bytes.Buffer
	observer := NewJSONLineObserver(&output, 100*time.Millisecond)
	observer.Observe(SpanObservation{Outcome: sqlstore.SpanOutcomeError, Acquired: true, WaitMS: 1, HoldMS: 1})
	if output.Len() != 0 {
		t.Fatalf("fast command error inside an acquired span emitted telemetry: %q", output.String())
	}
	observer.Observe(SpanObservation{Outcome: sqlstore.SpanOutcomeError, Acquired: true, HoldMS: 100})
	if !strings.Contains(output.String(), `"event":"issueops_record_span"`) {
		t.Fatalf("slow failed span not emitted: %q", output.String())
	}
}

func TestOptionalMillisecondsClampsNegativeAndKeepsNil(t *testing.T) {
	if optionalMilliseconds(nil) != nil {
		t.Fatal("nil duration became non-nil")
	}
	negative := -5 * time.Millisecond
	if got := optionalMilliseconds(&negative); got == nil || *got != 0 {
		t.Fatalf("negative duration = %v", got)
	}
	two := 2 * time.Millisecond
	if got := optionalMilliseconds(&two); got == nil || *got != 2 {
		t.Fatalf("duration = %v", got)
	}
}
