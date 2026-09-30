package installcli

import (
	claudeadapter "issueops/internal/adapter/claude"
	codexadapter "issueops/internal/adapter/codex"

	installadapter "issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"

	"issueops/internal/port"
)

func testClaudeInstaller() claudeadapter.Installer {
	return claudeadapter.NewInstaller(claudeadapter.Dependencies{
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
		WriteJSONPlan:                installutil.WriteJSONPlan})
}
func testCodexInstaller() codexadapter.Installer {
	return codexadapter.NewInstaller(codexadapter.Dependencies{
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
		TOMLString:                   installutil.TOMLString,
		ValidateHookConfigForMerge:   installutil.ValidateHookConfigForMerge,
		VerifyHookActivation:         installutil.VerifyHookActivation,
		WriteJSONPlan:                installutil.WriteJSONPlan,
		WriteTextPlan:                installutil.WriteTextPlan,
	})
}
