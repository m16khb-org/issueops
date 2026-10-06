package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	selfcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	workercontract "issueops/internal/contract/worker"
)

func summaryContent(t testing.TB, generated string, ok bool) string {
	t.Helper()
	snapshot := selfcontract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", GeneratedAt: generated, OK: ok}
	b, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func historyFixture(list statecontract.StateListResult, contents map[string]string) Service {
	return Service{
		Inspect: func(string) inspectcontract.InspectInfo { return inspectcontract.InspectInfo{} },
		Doctor: func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
			return doctorcontract.HarnessDoctorResult{OK: true}, nil
		},
		State: func() (statecontract.StateListResult, error) { return list, nil },
		StateRead: func(key string) (statecontract.StateResult, error) {
			content, ok := contents[key]
			if !ok {
				return statecontract.StateResult{}, errors.New("unreadable")
			}
			return statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Content: content}}, nil
		},
		Workers:       func() (workercontract.WorkerListResult, error) { return workercontract.WorkerListResult{OK: true}, nil },
		ResolveTarget: func(s string) string { return s },
	}
}

func TestRunSelectsLatestEligibleSummaryRegardlessOfOrder(t *testing.T) {
	contents := map[string]string{
		"self-verify-baseline":       summaryContent(t, "2026-09-28T00:00:00Z", true),
		"custom-failed-run":          summaryContent(t, "2026-09-30T00:00:00.123456789Z", false),
		"self-verify-candidates":     `{"schema_version":1,"kind":"self_verification_candidate_export","generated_at":"2030-01-01T00:00:00Z"}`,
		"self-verify-augment":        `{"schema_version":1,"kind":"self_augmentation_summary","generated_at":"2030-01-01T00:00:00Z"}`,
		"self-verify-future":         `{"schema_version":2,"kind":"self_verification_summary","generated_at":"2030-01-01T00:00:00Z"}`,
		"self-verify-missing-schema": `{"kind":"self_verification_summary","generated_at":"2030-01-01T00:00:00Z"}`,
		"self-verify-unrelated":      `{"ok":true}`,
		"self-verify-text":           `not json`,
	}
	records := []statecontract.StateListEntry{}
	for _, key := range []string{"self-verify-candidates", "self-verify-baseline", "custom-failed-run", "self-verify-augment", "self-verify-future", "self-verify-missing-schema", "self-verify-unrelated", "self-verify-text"} {
		records = append(records, statecontract.StateListEntry{Key: key, UpdatedAt: "2030-01-01T00:00:00Z", Bytes: len(contents[key])})
	}
	for i := 0; i < len(records); i++ {
		service := historyFixture(statecontract.StateListResult{OK: true, Records: records}, contents)
		lists, reads := 0, map[string]int{}
		originalList, originalRead := service.State, service.StateRead
		service.State = func() (statecontract.StateListResult, error) { lists++; return originalList() }
		service.StateRead = func(key string) (statecontract.StateResult, error) { reads[key]++; return originalRead(key) }
		before := append([]statecontract.StateListEntry(nil), records...)
		got := service.Run("repo")
		if !got.OK || !got.SelfVerify.Found || got.SelfVerify.LatestKey != "custom-failed-run" || got.SelfVerify.Bytes != len(contents["custom-failed-run"]) || got.SelfVerify.UpdatedAt != "2030-01-01T00:00:00Z" || len(got.Warnings) != 0 {
			t.Fatalf("rotation %d: %+v", i, got)
		}
		if lists != 1 || len(reads) != len(records) || !reflect.DeepEqual(before, records) {
			t.Fatalf("lists=%d reads=%v input mutated=%v", lists, reads, !reflect.DeepEqual(before, records))
		}
		for key, n := range reads {
			if n != 1 {
				t.Fatalf("%s read %d times", key, n)
			}
		}
		records = append(records[1:], records[0])
	}
}

func TestRunReusesHistoryTimestampOrdering(t *testing.T) {
	cases := []struct {
		name               string
		generated, updated []string
		want               string
		warnings           int
	}{
		{"nano and offset", []string{"2026-09-30T09:00:00.1+09:00", "2026-09-30T00:00:00.2Z"}, []string{"2030-01-01T00:00:00Z", "2020-01-01T00:00:00Z"}, "b", 0},
		{"valid before invalid", []string{"bad", "2020-01-01T00:00:00Z"}, []string{"2030-01-01T00:00:00Z", "2020-01-01T00:00:00Z"}, "b", 1},
		{"all invalid fallback", []string{"", "bad"}, []string{"2020-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}, "b", 2},
		{"valid updated fallback", []string{"", "bad"}, []string{"bad", "2026-01-01T00:00:00Z"}, "b", 2},
		{"same instant key tie", []string{"2026-09-30T09:00:00+09:00", "2026-09-30T00:00:00Z"}, []string{"2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}, "a", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			contents := map[string]string{"a": summaryContent(t, tc.generated[0], true), "b": summaryContent(t, tc.generated[1], false)}
			records := []statecontract.StateListEntry{{Key: "b", UpdatedAt: tc.updated[1]}, {Key: "a", UpdatedAt: tc.updated[0]}}
			for i := 0; i < 2; i++ {
				got := historyFixture(statecontract.StateListResult{OK: true, Records: records}, contents).Run("repo")
				if !got.OK || got.SelfVerify.LatestKey != tc.want || len(got.Warnings) != tc.warnings {
					t.Fatalf("%+v", got)
				}
				records[0], records[1] = records[1], records[0]
			}
		})
	}
}

func TestRunHistoryReadFailureAndDiagnosticsHaveDifferentOKEffects(t *testing.T) {
	list := statecontract.StateListResult{OK: true, Records: []statecontract.StateListEntry{{Key: "failed"}, {Key: "valid"}, {Key: "invalid-time"}}}
	contents := map[string]string{"valid": summaryContent(t, "2026-09-30T00:00:00Z", true), "invalid-time": summaryContent(t, "bad", true)}
	service := historyFixture(list, contents)
	service.Doctor = func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
		return doctorcontract.HarnessDoctorResult{OK: true}, errors.New("doctor error")
	}
	service.Workers = func() (workercontract.WorkerListResult, error) {
		return workercontract.WorkerListResult{OK: true}, errors.New("worker error")
	}
	got := service.Run("repo")
	want := []string{"doctor: doctor error", "workers: worker error", "selfverify: invalid_generated_at:invalid-time", "selfverify: failed: state_read:unreadable"}
	if got.OK || got.SelfVerify.LatestKey != "valid" || !reflect.DeepEqual(got.Warnings, want) {
		t.Fatalf("%+v", got)
	}
	service = historyFixture(list, contents)
	if got := service.Run("repo"); got.OK {
		t.Fatalf("read failure ignored: %+v", got)
	}
	contents["failed"] = `{"kind":"unrelated"}`
	got = service.Run("repo")
	if !got.OK || !reflect.DeepEqual(got.Warnings, []string{"selfverify: invalid_generated_at:invalid-time"}) {
		t.Fatalf("diagnostic changed OK: %+v", got)
	}
}

func TestRunSkipsHistoryReadsForEmptyOrFailedList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ok      bool
		err     error
		records []statecontract.StateListEntry
	}{
		{name: "empty", ok: true}, {name: "list error", ok: true, err: errors.New("list failed"), records: []statecontract.StateListEntry{{Key: "self-verify-latest"}}}, {name: "list not OK", records: []statecontract.StateListEntry{{Key: "self-verify-latest"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := historyFixture(statecontract.StateListResult{OK: tc.ok, Records: tc.records}, nil)
			service.State = func() (statecontract.StateListResult, error) {
				return statecontract.StateListResult{OK: tc.ok, Records: tc.records}, tc.err
			}
			service.StateRead = func(string) (statecontract.StateResult, error) {
				t.Fatal("unexpected read")
				return statecontract.StateResult{}, nil
			}
			got := service.Run("repo")
			if got.SelfVerify.Found || got.OK != (tc.ok && tc.err == nil) {
				t.Fatalf("%+v", got)
			}
		})
	}
	service := historyFixture(statecontract.StateListResult{OK: true, Records: []statecontract.StateListEntry{{Key: "self-verify-candidates"}}}, map[string]string{"self-verify-candidates": `{"schema_version":1,"kind":"self_verification_candidate_export"}`})
	if got := service.Run("repo"); !got.OK || got.SelfVerify.Found {
		t.Fatalf("candidate selected: %+v", got)
	}
}

func BenchmarkRunHistoryReads(b *testing.B) {
	for _, n := range []int{10, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			records := make([]statecontract.StateListEntry, n)
			contents := map[string]string{}
			for i := range records {
				key := fmt.Sprintf("run-%04d", i)
				records[i] = statecontract.StateListEntry{Key: key}
				contents[key] = summaryContent(b, "2026-09-30T00:00:00Z", true)
			}
			service := historyFixture(statecontract.StateListResult{OK: true, Records: records}, contents)
			reads := 0
			originalRead := service.StateRead
			service.StateRead = func(key string) (statecontract.StateResult, error) { reads++; return originalRead(key) }
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				service.Run("repo")
			}
			b.StopTimer()
			b.ReportMetric(float64(reads)/float64(b.N), "additional-reads/op")
		})
	}
}
