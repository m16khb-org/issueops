package daemon

import (
	contract "issueops/internal/contract/daemon"
	"testing"
	"time"
)

func TestStatusPredicatesRequireCompleteReadyIdentity(t *testing.T) {
	record := contract.InstanceRecord{PID: 17}
	ready := contract.Status{OK: true, Running: true, Reachable: true, IdentityVerified: true, Code: "ready", PID: 17, Instance: &record}
	if !IsReady(ready) || BlocksStart(ready) || !CanStop(ready) {
		t.Fatalf("valid ready: %+v", ready)
	}
	cases := map[string]func(*contract.Status){
		"failed observation": func(s *contract.Status) { s.OK = false },
		"not running":        func(s *contract.Status) { s.Running = false },
		"not reachable":      func(s *contract.Status) { s.Reachable = false },
		"not verified":       func(s *contract.Status) { s.IdentityVerified = false },
		"wrong code":         func(s *contract.Status) { s.Code = "unknown" },
		"no instance":        func(s *contract.Status) { s.Instance = nil },
		"wrong pid":          func(s *contract.Status) { s.PID = 18 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			s := ready
			change(&s)
			if IsReady(s) || !BlocksStart(s) || CanStop(s) {
				t.Fatalf("unverified status accepted: %+v", s)
			}
		})
	}
	stopped := contract.Status{OK: true, Code: "stopped"}
	if IsReady(stopped) || BlocksStart(stopped) || CanStop(stopped) {
		t.Fatalf("stopped: %+v", stopped)
	}
	socketless := contract.Status{Running: true, Code: "socket_unreachable", PID: 17, Instance: &record}
	if IsReady(socketless) || !BlocksStart(socketless) || !CanStop(socketless) {
		t.Fatalf("socketless verified candidate: %+v", socketless)
	}
	socketless.Instance = nil
	if CanStop(socketless) {
		t.Fatal("socketless process without recorded identity was accepted")
	}
}

func TestProcessIdentityMatchesPreservesPlatformStabilityAndStartTicks(t *testing.T) {
	record := contract.InstanceRecord{ProcessStartTime: "2026-07-31T07:14:57.50Z", Executable: "/original"}
	observed := contract.ProcessIdentity{StartTime: record.ProcessStartTime, Executable: "/updated"}
	if !ProcessIdentityMatches(record, observed, time.UTC) {
		t.Fatal("unstable executable projection changed identity")
	}
	observed.ExecutablePathStable = true
	if ProcessIdentityMatches(record, observed, time.UTC) {
		t.Fatal("stable executable mismatch was accepted")
	}
	observed.Executable = record.Executable
	if !ProcessIdentityMatches(record, observed, time.UTC) {
		t.Fatal("same process rejected")
	}
	observed.StartTime = "2026-07-31T07:14:57.51Z"
	if ProcessIdentityMatches(record, observed, time.UTC) {
		t.Fatal("different start ticks were accepted")
	}
}

func TestProcessStartTimeNormalizationUsesExplicitLocation(t *testing.T) {
	seoul := time.FixedZone("fixture", 9*60*60)
	for _, value := range []string{"2026년  7월 31일 금요일 16시 14분 57초", "Fri Jul 31 16:14:57 2026"} {
		got, err := CanonicalProcessStartTime(value, seoul)
		if err != nil || got != "2026-07-31T07:14:57Z" {
			t.Fatalf("normalize %q: %q %v", value, got, err)
		}
		if !ProcessStartTimeEqual(value, "2026-07-31T07:14:57Z", seoul) {
			t.Fatalf("equivalent identity rejected: %q", value)
		}
		if ProcessStartTimeEqual(value, "2026-07-31T07:14:57Z", time.UTC) {
			t.Fatalf("location ignored: %q", value)
		}
	}
	if ProcessStartTimeEqual("linux:boot:100", "linux:boot:101", seoul) {
		t.Fatal("different Linux clock ticks accepted")
	}
	if _, err := CanonicalProcessStartTime("2026년  2월 31일 화요일 16시 14분 57초", seoul); err == nil {
		t.Fatal("invalid calendar date accepted")
	}
}
