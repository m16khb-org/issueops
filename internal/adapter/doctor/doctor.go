package doctor

import (
	doctorapp "issueops/internal/application/doctor"
	"time"
)

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	return (doctorapp.Service{Effects: doctorapp.Effects{
		NormalizeRoot:              NormalizeRepoRoot,
		StateDir:                   StateDir,
		StateDoctor:                StateDoctor,
		ValidateLifecycle:          ValidateProjectLifecycleState,
		CheckProjectDocs:           checkProjectDocs,
		CheckRepoLocalRuntimeState: checkRepoLocalRuntimeState,
		CheckLoopContracts:         checkLoopContracts,
		CheckPipeCapacity:          checkPipeCapacity,
		CheckMCPGateways:           checkMCPGateways,
		CheckNativeIntegrations:    checkNativeIntegrations,
		CheckBinaryDrift:           checkBinaryDrift,
		Now:                        time.Now,
	}}).Run(req)
}

func doctorHealthy(checks []HarnessDoctorCheck, issues []HarnessDoctorIssue) bool {
	return doctorapp.DoctorHealthy(checks, issues)
}
func checkDaemonAdmission(result *HarnessDoctorResult, admission HarnessDoctorDaemonAdmission) {
	doctorapp.CheckDaemonAdmission(result, admission)
}
func addCheck(result *HarnessDoctorResult, name string, healthy bool, summary string) {
	doctorapp.AddCheck(result, name, healthy, summary)
}
func addIssue(result *HarnessDoctorResult, code, severity, summary, path string, fix *HarnessDoctorFix) {
	doctorapp.AddIssue(result, code, severity, summary, path, fix)
}
