package doctor

import (
	doctorcontract "issueops/internal/contract/doctor"
	doctordomain "issueops/internal/domain/doctor"
	"testing"
)

func TestDoctorHealthyFollowsChecksAndIssues(t *testing.T) {
	tests := []struct {
		name   string
		checks []doctorcontract.HarnessDoctorCheck
		issues []doctorcontract.HarnessDoctorIssue
		want   bool
	}{
		{name: "healthy checks", checks: []doctorcontract.HarnessDoctorCheck{{Name: "ready", Healthy: true}}, want: true},
		{name: "unhealthy check without issue", checks: []doctorcontract.HarnessDoctorCheck{{Name: "ready", Healthy: false}}, want: false},
		{name: "warning issue", issues: []doctorcontract.HarnessDoctorIssue{{Code: "warning", Severity: "warning"}}, want: false},
		{name: "informational issue", issues: []doctorcontract.HarnessDoctorIssue{{Code: "info", Severity: "info"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doctordomain.Healthy(tt.checks, tt.issues); got != tt.want {
				t.Fatalf("doctorHealthy()=%t want %t", got, tt.want)
			}
		})
	}
}
