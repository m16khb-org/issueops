package adapter_test

import (
	agyadapter "issueops/internal/adapter/agy"
	claudeadapter "issueops/internal/adapter/claude"
	codexadapter "issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	installadapter "issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	omoadapter "issueops/internal/adapter/omo"
	"issueops/internal/port"
)

func testAgyInstaller() agyadapter.Installer {
	return agyadapter.NewInstaller(agyadapter.Dependencies{
		MergeJSONMapFile:                installutil.MergeJSONMapFile,
		VerifyJSONMapEntry:              installutil.VerifyJSONMapEntry,
		CaptureNativeActivationEvidence: installutil.CaptureNativeActivationEvidence,
		MCPCatalogSHA256:                func() (string, error) { return installutil.SemanticSHA256(mcpcatalog.AdvertisedTools()) },
		NewInstallPlan:                  func(host string, dry bool) port.InstallPlan { return installutil.NewPlan(host, dry) },
		PlanHostSkillLinks:              installutil.PlanHostSkillLinks,
		SemanticSHA256:                  installutil.SemanticSHA256,
		WriteJSONPlan:                   installutil.WriteJSONPlan})
}
func testClaudeInstaller() claudeadapter.Installer {
	return claudeadapter.NewInstaller(claudeadapter.Dependencies{
		MergeJSONMapFile:                installutil.MergeJSONMapFile,
		VerifyJSONMapEntry:              installutil.VerifyJSONMapEntry,
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
func testOmoInstaller() omoadapter.Installer {
	return omoadapter.NewInstaller(omoadapter.Dependencies{
		MergeJSONMapFile:                installutil.MergeJSONMapFile,
		VerifyJSONMapEntry:              installutil.VerifyJSONMapEntry,
		CaptureNativeActivationEvidence: installutil.CaptureNativeActivationEvidence,
		MCPCatalogSHA256:                func() (string, error) { return installutil.SemanticSHA256(mcpcatalog.AdvertisedTools()) },
		NewInstallPlan:                  func(host string, dry bool) port.InstallPlan { return installutil.NewPlan(host, dry) },
		PlanHostSkillLinks:              installutil.PlanHostSkillLinks,
		SemanticSHA256:                  installutil.SemanticSHA256,
		WriteJSONPlan:                   installutil.WriteJSONPlan,
		WriteTextPlan:                   installutil.WriteTextPlan,
	}, hostprotocol.OmoLifecycleExtension)
}
