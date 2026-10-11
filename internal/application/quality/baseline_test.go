package quality

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"strings"
	"testing"

	contract "issueops/internal/contract/quality"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/quality"
)

type memState struct {
	records map[string]string
	readErr error
	wrote   int
}

func (m *memState) read(key string) (state.StateResult, error) {
	if m.readErr != nil {
		return state.StateResult{}, m.readErr
	}
	content, ok := m.records[key]
	if !ok {
		return state.StateResult{}, fs.ErrNotExist
	}
	return state.StateResult{OK: true, Record: state.RecordEnvelope{Content: content}}, nil
}
func (m *memState) write(key, content string) (state.StateResult, error) {
	m.wrote++
	m.records[key] = content
	return state.StateResult{OK: true}, nil
}
func (m *memState) store() SNRBaselineStore {
	return SNRBaselineStore{
		CanonicalRepository: func(root string) (string, error) { return "canon:" + root, nil },
		ReadState:           m.read,
		WriteState:          m.write,
	}
}

func TestSNRBaselineSaveThenReadRoundTripsPerRepository(t *testing.T) {
	m := &memState{records: map[string]string{}}
	store := m.store()
	if _, present, err := store.Read("/a"); err != nil || present {
		t.Fatalf("missing baseline present=%t err=%v", present, err)
	}
	if err := store.Save("/a", 0.75); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("/b", 0.25); err != nil {
		t.Fatal(err)
	}
	if ratio, present, err := store.Read("/a"); err != nil || !present || ratio != 0.75 {
		t.Fatalf("ratio=%v present=%t err=%v", ratio, present, err)
	}
	if ratio, _, _ := store.Read("/b"); ratio != 0.25 || len(m.records) != 2 {
		t.Fatalf("repositories not isolated: ratio=%v records=%d", ratio, len(m.records))
	}
}

func TestSNRBaselineSaveRejectsInvalidRatioWithoutWriting(t *testing.T) {
	m := &memState{records: map[string]string{}}
	for _, ratio := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1)} {
		if err := m.store().Save("/a", ratio); err == nil {
			t.Fatalf("ratio %v accepted", ratio)
		}
	}
	if m.wrote != 0 {
		t.Fatalf("wrote %d times", m.wrote)
	}
}

func TestSNRBaselineUnconfiguredStoreFailsClosed(t *testing.T) {
	store := SNRBaselineStore{CanonicalRepository: func(s string) (string, error) { return s, nil }}
	if _, _, err := store.Read("/a"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("read err=%v", err)
	}
	if err := store.Save("/a", 0.5); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("save err=%v", err)
	}
}

func TestSNRBaselineReadFailsClosed(t *testing.T) {
	canonErr := errors.New("no repo")
	key := domain.SNRBaselineKey("canon:/a")
	foreign, _ := json.Marshal(contract.SNRBaselineRecord{SchemaVersion: contract.SNRBaselineSchemaVersion, Repository: "other", Ratio: 0.5})
	for name, content := range map[string]string{"garbage": "{nope", "foreign": string(foreign)} {
		t.Run("rejects "+name+" record", func(t *testing.T) {
			m := &memState{records: map[string]string{key: content}}
			if _, present, err := m.store().Read("/a"); err == nil || present {
				t.Fatalf("accepted: present=%t err=%v", present, err)
			}
		})
	}
	t.Run("propagates state read error", func(t *testing.T) {
		m := &memState{records: map[string]string{}, readErr: errors.New("io")}
		if _, _, err := m.store().Read("/a"); err == nil || err.Error() != "io" {
			t.Fatalf("read err=%v", err)
		}
	})
	t.Run("propagates identity error on read and save", func(t *testing.T) {
		store := (&memState{records: map[string]string{}}).store()
		store.CanonicalRepository = func(string) (string, error) { return "", canonErr }
		if _, _, err := store.Read("/a"); !errors.Is(err, canonErr) {
			t.Fatalf("read identity err=%v", err)
		}
		if err := store.Save("/a", 0.5); !errors.Is(err, canonErr) {
			t.Fatalf("save identity err=%v", err)
		}
	})
	t.Run("rejects result without record", func(t *testing.T) {
		empty := SNRBaselineStore{
			CanonicalRepository: func(s string) (string, error) { return s, nil },
			ReadState:           func(string) (state.StateResult, error) { return state.StateResult{}, nil },
		}
		if _, present, err := empty.Read("/a"); err == nil || present {
			t.Fatalf("no-record result accepted: present=%t err=%v", present, err)
		}
	})
}

func inspectStub(signals []contract.Signal) func(string) contract.InspectResult {
	return func(string) contract.InspectResult {
		return contract.InspectResult{OK: true, HealthStatus: contract.HealthStatusHealthy, GateStatus: contract.GateStatusPass, Signals: signals}
	}
}

func TestInspectWithBaselineSavesCurrentSNRAndFlagsRegression(t *testing.T) {
	var saved []float64
	deps := InspectBaselineDeps{
		Inspect:         inspectStub([]contract.Signal{{ID: "code-snr", Status: "ok", Value: 0.5}}),
		ReadSNRBaseline: func(string) (float64, bool, error) { return 0.8, true, nil },
		SaveSNRBaseline: func(_ string, ratio float64) error { saved = append(saved, ratio); return nil },
	}
	result, baseline, present := InspectWithBaseline(InspectRequest{Root: "/r", Trend: true, SaveBaseline: true}, deps)
	if baseline != 0.8 || !present || len(saved) != 1 || saved[0] != 0.5 {
		t.Fatalf("baseline=%v present=%t saved=%v", baseline, present, saved)
	}
	if result.GateStatus != contract.GateStatusBlock || len(result.Findings) != 1 || result.Findings[0].ID != "code-snr-regression" {
		t.Fatalf("result=%+v", result)
	}
}

func TestInspectWithBaselineWithoutTrendNeitherReadsNorFlags(t *testing.T) {
	deps := InspectBaselineDeps{
		Inspect:         inspectStub([]contract.Signal{{ID: "code-snr", Status: "ok", Value: 0.1}}),
		ReadSNRBaseline: func(string) (float64, bool, error) { t.Fatal("baseline read without trend"); return 0, false, nil },
		SaveSNRBaseline: func(string, float64) error { t.Fatal("baseline saved unrequested"); return nil },
	}
	result, _, present := InspectWithBaseline(InspectRequest{Root: "/r"}, deps)
	if present || len(result.Findings) != 0 || result.GateStatus != contract.GateStatusPass {
		t.Fatalf("result=%+v", result)
	}
}

func TestInspectWithBaselineReportsCollectorFailuresForReadSaveAndMissingSignal(t *testing.T) {
	deps := InspectBaselineDeps{
		Inspect:         inspectStub(nil),
		ReadSNRBaseline: func(string) (float64, bool, error) { return 0, false, errors.New("unreadable") },
		SaveSNRBaseline: func(string, float64) error { t.Fatal("saved with unavailable signal"); return nil },
	}
	result, _, present := InspectWithBaseline(InspectRequest{Root: "/r", Trend: true, SaveBaseline: true}, deps)
	want := []string{"read-baseline: unreadable", "save-baseline: code-snr signal is unavailable"}
	if present || len(result.Warnings) != 2 || result.Warnings[0] != want[0] || result.Warnings[1] != want[1] {
		t.Fatalf("warnings=%q present=%t", result.Warnings, present)
	}
	deps.Inspect = inspectStub([]contract.Signal{{ID: "code-snr", Status: "ok", Value: 0.4}})
	deps.ReadSNRBaseline = func(string) (float64, bool, error) { return 0, false, nil }
	deps.SaveSNRBaseline = func(string, float64) error { return errors.New("disk full") }
	result, _, _ = InspectWithBaseline(InspectRequest{Root: "/r", Trend: true, SaveBaseline: true}, deps)
	if len(result.Warnings) != 1 || result.Warnings[0] != "save-baseline: disk full" {
		t.Fatalf("warnings=%q", result.Warnings)
	}
}
