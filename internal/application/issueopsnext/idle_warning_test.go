package issueopsnext

import (
	"context"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func TestNextAddsIdleWarningOnBothSelectionPaths(t *testing.T) {
	now := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	const marker = "has had no record write for"
	idleLines := func(warnings []string) int {
		count := 0
		for _, warning := range warnings {
			if strings.Contains(warning, marker) {
				count++
			}
		}
		return count
	}

	for _, scenario := range []struct {
		name     string
		ago      time.Duration
		explicit bool
		want     int
	}{
		{"cwd path 8 days", 8 * day, false, 1},
		{"explicit id 8 days", 8 * day, true, 1},
		{"cwd path 6 days", 6 * day, false, 0},
		{"explicit id 6 days", 6 * day, true, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			record := issueopscontract.Record{
				ID: "io-a", Repo: "/repo", Branch: "12-a", Phase: issueopscontract.PhaseGrill,
				UpdatedAt: now.Add(-scenario.ago).Format(time.RFC3339Nano),
			}
			entries := []issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: record.Phase, Branch: "12-a"}}
			ports := testPorts(entries, map[string]issueopscontract.Record{"io-a": record})
			clockReads := 0
			ports.Now = func() time.Time {
				clockReads++
				return now
			}
			ports.ReadSelected = func(context.Context, string, string) (issueopscontract.Record, error) {
				return record, nil
			}
			ports.ScanRecords = func(context.Context, string, func(issueopscontract.Record) error) error { return nil }
			id := ""
			if scenario.explicit {
				id = "io-a"
			}

			result, err := NewService(ports).Next(context.Background(), "/state", "/repo", id)

			if err != nil || result.Selected == nil || result.Selected.ID != "io-a" {
				t.Fatalf("selection = %+v, err = %v", result, err)
			}
			if got := idleLines(result.Warnings); got != scenario.want {
				t.Fatalf("idle warnings = %d, want %d: %q", got, scenario.want, result.Warnings)
			}
			if clockReads != 1 {
				t.Fatalf("Now calls = %d, want 1", clockReads)
			}
			if result.GeneratedAt != "2026-11-11T00:00:00Z" {
				t.Fatalf("GeneratedAt = %q", result.GeneratedAt)
			}
		})
	}
}

// TestNextOmitsIdleWarningWithoutSelection pins that a cycle next cannot select
// never gets the stall warning, even when its record is long idle and readable.
func TestNextOmitsIdleWarningWithoutSelection(t *testing.T) {
	now := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	idle := now.Add(-10 * 24 * time.Hour).Format(time.RFC3339Nano)
	for _, scenario := range []struct {
		name  string
		id    string
		ports func() Ports
	}{
		{"foreign explicit id", "io-selected", func() Ports {
			ports := testPorts(nil, nil)
			// Same fence condition as TestNextSelectedRepositoryFence's foreign row.
			ports.SourceRoot = func(path string) string { return path }
			ports.ReadSelected = func(context.Context, string, string) (issueopscontract.Record, error) {
				return issueopscontract.Record{ID: "io-selected", Repo: "/foreign", Phase: issueopscontract.PhaseGrill, UpdatedAt: idle}, nil
			}
			return ports
		}},
		{"invalid cwd entry", "", func() Ports {
			return testPorts(
				[]issueopsinventorycontract.ListEntry{{ID: "io-bad", Invalid: true}},
				map[string]issueopscontract.Record{"io-bad": {ID: "io-bad", Repo: "/repo", Phase: issueopscontract.PhaseGrill, UpdatedAt: idle}},
			)
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ports := scenario.ports()
			ports.Now = func() time.Time { return now }

			result, err := NewService(ports).Next(context.Background(), "/state", "/repo", scenario.id)

			if err != nil || result.Stage.Key != "invalid" {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			for _, warning := range result.Warnings {
				if strings.Contains(warning, "has had no record write for") {
					t.Fatalf("unselected cycle got the idle warning: %q", result.Warnings)
				}
			}
		})
	}
}
