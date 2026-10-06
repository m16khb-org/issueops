package doctor

import (
	doctordomain "issueops/internal/domain/doctor"
)

const pipeCapacityWarningThreshold = doctordomain.PipeCapacityWarningThreshold
const mcpGatewayFDWarningThreshold = doctordomain.MCPGatewayFDWarningThreshold

func doctorHealthy(checks []HarnessDoctorCheck, issues []HarnessDoctorIssue) bool {
	return doctordomain.Healthy(checks, issues)
}
