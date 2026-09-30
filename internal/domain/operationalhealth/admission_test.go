package operationalhealth

import "testing"

func TestDecideAdmissionDistinguishesInconsistentAndSaturated(t *testing.T) {
	inconsistent := DecideAdmission(Admission{Observed: true, Active: 1, Maximum: 2, Accepting: false})
	if inconsistent.Healthy || inconsistent.IssueCode != "daemon_admission_inconsistent" {
		t.Fatalf("unexpected inconsistent decision: %+v", inconsistent)
	}
	saturated := DecideAdmission(Admission{Observed: true, Active: 2, Maximum: 2, Accepting: false})
	if saturated.Healthy || saturated.IssueCode != "daemon_connection_limit_reached" {
		t.Fatalf("unexpected saturated decision: %+v", saturated)
	}
	draining := DecideAdmission(Admission{Observed: true, Active: 2, Maximum: 2, Accepting: false, Draining: true})
	if !draining.Healthy || draining.IssueCode != "" {
		t.Fatalf("draining must not be reported saturated: %+v", draining)
	}
}
