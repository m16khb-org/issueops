package doctor

import (
	"testing"

	doctorcontract "issueops/internal/contract/doctor"
)

func TestAdmissionFlagsInconsistentTelemetry(t *testing.T) {
	result := doctorcontract.HarnessDoctorResult{Checks: []doctorcontract.HarnessDoctorCheck{}, Issues: []doctorcontract.HarnessDoctorIssue{}}
	CheckDaemonAdmission(&result, doctorcontract.HarnessDoctorDaemonAdmission{Observed: true, MaxConnections: 2, ActiveConnections: 1, Accepting: false})
	if len(result.Issues) != 1 || result.Issues[0].Code != "daemon_admission_inconsistent" || result.Checks[0].Healthy {
		t.Fatalf("unexpected admission result: %+v", result)
	}
}
