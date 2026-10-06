package doctor

import "testing"

func TestDoctorHealthyFollowsChecksAndIssues(t *testing.T) {
	tests := []struct {
		name   string
		checks []HarnessDoctorCheck
		issues []HarnessDoctorIssue
		want   bool
	}{
		{name: "healthy checks", checks: []HarnessDoctorCheck{{Name: "ready", Healthy: true}}, want: true},
		{name: "unhealthy check without issue", checks: []HarnessDoctorCheck{{Name: "ready", Healthy: false}}, want: false},
		{name: "warning issue", issues: []HarnessDoctorIssue{{Code: "warning", Severity: "warning"}}, want: false},
		{name: "informational issue", issues: []HarnessDoctorIssue{{Code: "info", Severity: "info"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doctorHealthy(tt.checks, tt.issues); got != tt.want {
				t.Fatalf("doctorHealthy()=%t want %t", got, tt.want)
			}
		})
	}
}
