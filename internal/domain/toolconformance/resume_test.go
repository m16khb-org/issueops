package toolconformance

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/toolconformance"
)

func TestValidatePreviousSelectionRejectsDuplicateHosts(t *testing.T) {
	previous := &contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "codex"}, {Host: "codex"}}}
	err := ValidatePreviousSelection(previous, []SelectedFixturePair{{Host: "codex", Fixture: contract.Fixture{ID: "fixture"}}})
	if err == nil || err.Error() != "invalid_previous_report_identity" {
		t.Fatalf("duplicate host accepted: %v", err)
	}
}

func resumeFixtures() []contract.Fixture {
	return []contract.Fixture{{ID: "f1", SchemaSHA256: "schema"}}
}

func completedHostReport() contract.BenchmarkReport {
	episode := validEpisode()
	return contract.BenchmarkReport{Hosts: []contract.HostReport{{
		Status:            contract.HostStatusSupported,
		Evidence:          contract.HostEvidence{Installed: true, PreflightReady: true, LiveAttempted: true, LiveVerified: true},
		Host:              "codex",
		Version:           "1.0",
		RequestedModel:    "req",
		ObservedModel:     "obs",
		AttemptCount:      1,
		CompletedEpisodes: 1,
		Cases:             []contract.EpisodeReport{episode},
	}}}
}

func resume(report *contract.BenchmarkReport, target int) (*contract.HostReport, error) {
	selected := []SelectedFixturePair{{Host: "codex", Fixture: resumeFixtures()[0]}}
	return ResumeHostReport(report, "codex", "req", "1.0", "p", resumeFixtures(), selected, target)
}

func TestResumeHostReportReturnsNilWithoutPreviousEvidence(t *testing.T) {
	got, err := resume(nil, 3)
	if got != nil || err != nil {
		t.Fatalf("nil report: %v %v", got, err)
	}
	other := &contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "claude"}}}
	got, err = resume(other, 3)
	if got != nil || err != nil {
		t.Fatalf("other host only: %v %v", got, err)
	}
}

func TestResumeHostReportAcceptsConsistentCompletedEvidence(t *testing.T) {
	report := completedHostReport()
	got, err := resume(&report, 1)
	if err != nil || got == nil || got.Host != "codex" || len(got.Cases) != 1 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestResumeHostReportAcceptsIncompleteOnlyEvidence(t *testing.T) {
	incomplete := validEpisode()
	incomplete.Status = contract.EpisodeIncomplete
	incomplete.ObservedModel = ""
	report := completedHostReport()
	host := &report.Hosts[0]
	host.Status = contract.HostStatusUnavailable
	host.ObservedModel = ""
	host.CompletedEpisodes = 0
	host.Evidence.LiveVerified = false
	host.Evidence.StatusReason = "live_probe_incomplete"
	host.Cases = []contract.EpisodeReport{incomplete}
	if got, err := resume(&report, 1); err != nil || got == nil {
		t.Fatalf("incomplete-only evidence rejected: %v", err)
	}
}

func TestResumeHostReportRejectsInvalidPreviousEvidence(t *testing.T) {
	const invalid = "invalid_previous_episode_evidence"
	cases := map[string]func(r *contract.BenchmarkReport){
		"version mismatch":           func(r *contract.BenchmarkReport) { r.Hosts[0].Version = "9" },
		"requested model mismatch":   func(r *contract.BenchmarkReport) { r.Hosts[0].RequestedModel = "other" },
		"attempt count drift":        func(r *contract.BenchmarkReport) { r.Hosts[0].AttemptCount = 2 },
		"completed count drift":      func(r *contract.BenchmarkReport) { r.Hosts[0].CompletedEpisodes = 0 },
		"unknown fixture":            func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].FixtureID = "nope" },
		"episode schema drift":       func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].SchemaSHA256 = "other" },
		"episode profile mismatch":   func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].Profile = "other" },
		"episode host mismatch":      func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].Host = "claude" },
		"episode attempt zero":       func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].Attempt = 0 },
		"padded observed model":      func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].ObservedModel = " obs" },
		"completed lacks live proof": func(r *contract.BenchmarkReport) { r.Hosts[0].Cases[0].DurationMS = 0 },
		"host observed model drift":  func(r *contract.BenchmarkReport) { r.Hosts[0].ObservedModel = "other" },
		"host not installed":         func(r *contract.BenchmarkReport) { r.Hosts[0].Evidence.Installed = false },
		"preflight not ready":        func(r *contract.BenchmarkReport) { r.Hosts[0].Evidence.PreflightReady = false },
		"live verified lost":         func(r *contract.BenchmarkReport) { r.Hosts[0].Evidence.LiveVerified = false },
		"supported with reason":      func(r *contract.BenchmarkReport) { r.Hosts[0].Evidence.StatusReason = "x" },
		"completed but unavailable":  func(r *contract.BenchmarkReport) { r.Hosts[0].Status = contract.HostStatusUnavailable },
	}
	for name, mutate := range cases {
		report := completedHostReport()
		mutate(&report)
		got, err := resume(&report, 1)
		if got != nil || err == nil || err.Error() != invalid {
			t.Errorf("%s: got %v err %v", name, got, err)
		}
	}
}

func TestResumeHostReportRejectsIncompleteHostWithWrongStatus(t *testing.T) {
	incomplete := validEpisode()
	incomplete.Status = contract.EpisodeIncomplete
	incomplete.ObservedModel = ""
	for name, mutate := range map[string]func(h *contract.HostReport){
		"supported status":   func(h *contract.HostReport) { h.Status = contract.HostStatusSupported },
		"missing reason":     func(h *contract.HostReport) { h.Evidence.StatusReason = "" },
		"live attempt unset": func(h *contract.HostReport) { h.Evidence.LiveAttempted = false },
	} {
		report := completedHostReport()
		host := &report.Hosts[0]
		host.Status = contract.HostStatusUnavailable
		host.ObservedModel = ""
		host.CompletedEpisodes = 0
		host.Evidence.LiveVerified = false
		host.Evidence.StatusReason = "live_probe_incomplete"
		host.Cases = []contract.EpisodeReport{incomplete}
		mutate(host)
		if got, err := resume(&report, 1); got != nil || err == nil {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
}

func TestResumeHostReportRejectsEmptyCaseList(t *testing.T) {
	report := completedHostReport()
	host := &report.Hosts[0]
	host.Cases, host.AttemptCount, host.CompletedEpisodes = nil, 0, 0
	host.Status = contract.HostStatusUnavailable
	host.ObservedModel = ""
	host.Evidence.LiveAttempted, host.Evidence.LiveVerified = false, false
	host.Evidence.StatusReason = "live_probe_incomplete"
	if got, err := resume(&report, 1); got != nil || err == nil {
		t.Fatalf("host with no live attempts accepted: %v", err)
	}
}

func TestResumeHostReportRejectsDuplicateIdentityAndEvidence(t *testing.T) {
	report := completedHostReport()
	host := &report.Hosts[0]
	host.Cases = append(host.Cases, host.Cases[0])
	host.AttemptCount, host.CompletedEpisodes = 2, 2
	if _, err := resume(&report, 5); err == nil || err.Error() != "duplicate_previous_episode_identity" {
		t.Fatalf("duplicate identity: %v", err)
	}

	report = completedHostReport()
	host = &report.Hosts[0]
	second := host.Cases[0]
	second.Attempt = 2
	host.Cases = append(host.Cases, second)
	host.AttemptCount, host.CompletedEpisodes = 2, 2
	if _, err := resume(&report, 5); err == nil || err.Error() != "duplicate_previous_episode_evidence" {
		t.Fatalf("duplicate evidence id: %v", err)
	}
}

func TestResumeHostReportRejectsConflictingObservedModels(t *testing.T) {
	report := completedHostReport()
	host := &report.Hosts[0]
	second := validEpisode()
	second.Attempt = 2
	second.EvidenceID = evidenceID("4")
	second.ObservedModel = "different"
	host.Cases = append(host.Cases, second)
	host.AttemptCount, host.CompletedEpisodes = 2, 2
	if got, err := resume(&report, 5); got != nil || err == nil || err.Error() != "invalid_previous_episode_evidence" {
		t.Fatalf("conflicting observed models: %v %v", got, err)
	}
}

func TestResumeHostReportRejectsRecordedAttemptsAboveTarget(t *testing.T) {
	report := completedHostReport()
	if got, err := resume(&report, 0); got != nil || err == nil || err.Error() != "invalid_previous_episode_evidence" {
		t.Fatalf("completed above target accepted: %v %v", got, err)
	}
}

func TestValidatePreviousSelectionReportsStableErrorCodes(t *testing.T) {
	selected := []SelectedFixturePair{{Host: "codex", Fixture: contract.Fixture{ID: "f1"}}}
	for want, previous := range map[string]*contract.BenchmarkReport{
		"invalid_previous_report_identity":   {Hosts: []contract.HostReport{{Host: ""}}},
		"invalid_previous_episode_selection": {Hosts: []contract.HostReport{{Host: "claude"}}},
	} {
		if err := ValidatePreviousSelection(previous, selected); err == nil || err.Error() != want {
			t.Errorf("want %s, got %v", want, err)
		}
	}
	other := &contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "codex", Cases: []contract.EpisodeReport{{FixtureID: "f2"}}}}}
	if err := ValidatePreviousSelection(other, selected); err == nil || err.Error() != "invalid_previous_episode_selection" {
		t.Fatalf("unselected fixture: %v", err)
	}
}

func TestResumeHostReportRejectsUnknownEpisodeStatus(t *testing.T) {
	episode := validEpisode()
	episode.Status = "weird"
	episode.ObservedModel = ""
	report := completedHostReport()
	host := &report.Hosts[0]
	host.Status = contract.HostStatusUnavailable
	host.ObservedModel = ""
	host.CompletedEpisodes = 0
	host.Evidence.LiveVerified = false
	host.Evidence.StatusReason = "live_probe_incomplete"
	host.Cases = []contract.EpisodeReport{episode}
	if got, err := resume(&report, 1); got != nil || err == nil || err.Error() != "invalid_previous_episode_evidence" {
		t.Fatalf("unknown status accepted: %v %v", got, err)
	}
}

func TestResumeHostReportAcceptsObservedModelAtMaximumLength(t *testing.T) {
	report := completedHostReport()
	host := &report.Hosts[0]
	model := strings.Repeat("m", 256)
	host.ObservedModel = model
	host.Cases[0].ObservedModel = model
	if got, err := resume(&report, 1); err != nil || got == nil {
		t.Fatalf("256-byte observed model rejected: %v", err)
	}
}

func TestResumeHostReportRejectsEmptyCaseListEvenWhenLiveAttemptIsClaimed(t *testing.T) {
	report := completedHostReport()
	host := &report.Hosts[0]
	host.Cases, host.AttemptCount, host.CompletedEpisodes = nil, 0, 0
	host.Status = contract.HostStatusUnavailable
	host.ObservedModel = ""
	host.Evidence.LiveAttempted, host.Evidence.LiveVerified = true, false
	host.Evidence.StatusReason = "live_probe_incomplete"
	if got, err := resume(&report, 1); got != nil || err == nil {
		t.Fatalf("claimed live attempt without cases accepted: %v", err)
	}
}
