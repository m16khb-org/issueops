package claude

import (
	installadapter "issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	"issueops/internal/port"
)

func testDependencies() Dependencies {
	return Dependencies{
		CaptureNativeActivationEvidence: installutil.CaptureNativeActivationEvidence,
		FileBuildGenerationString:       installadapter.FileBuildGenerationString,
		HookGroupContainsAgentHarness:   installutil.HookGroupContainsAgentHarness,
		HookGroupContainsCommand:        installutil.HookGroupContainsCommand,
		HookTargetDriftMessages:         installutil.HookTargetDriftMessages,
		HookTargetGenerationMessages: func(config map[string]any, host, expected, running string, read func(string) string) []string {
			return installutil.HookTargetGenerationMessages(config, host, expected, running, read)
		},
		NewInstallPlan:               func(host string, dry bool) port.InstallPlan { return installutil.NewPlan(host, dry) },
		PlanHostSkillLinks:           installutil.PlanHostSkillLinks,
		RunningBuildGenerationString: installadapter.RunningBuildGenerationString,
		SemanticSHA256:               installutil.SemanticSHA256,
		ValidateHookConfigForMerge:   installutil.ValidateHookConfigForMerge,
		VerifyHookActivation:         installutil.VerifyHookActivation,
		WriteJSONPlan:                installutil.WriteJSONPlan}
}
func testInstaller() Installer { return NewInstaller(testDependencies()) }
