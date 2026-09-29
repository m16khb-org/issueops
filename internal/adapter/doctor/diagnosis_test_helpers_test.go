package doctor

import (
	doctorapp "issueops/internal/application/doctor"
	doctordomain "issueops/internal/domain/doctor"
)

const pipeCapacityWarningThreshold = doctordomain.PipeCapacityWarningThreshold
const mcpGatewayFDWarningThreshold = doctordomain.MCPGatewayFDWarningThreshold

func doctorHealthy(checks []HarnessDoctorCheck, issues []HarnessDoctorIssue) bool {
	return doctordomain.Healthy(checks, issues)
}
func checkDaemonAdmission(result *HarnessDoctorResult, admission HarnessDoctorDaemonAdmission) {
	doctorapp.CheckDaemonAdmission(result, admission)
}
