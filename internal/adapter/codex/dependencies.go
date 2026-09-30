package codex

import (
	"issueops/internal/port"
	"os"
)

// Dependencies contains the file and host capabilities used by one installer.
type Dependencies struct {
	CaptureNativeActivationEvidence func(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error)
	FileBuildGenerationString       func(path string) string
	HookGroupContainsAgentHarness   func(group any) bool
	HookGroupContainsCommand        func(group any, commandPrefix string) bool
	HookTargetDriftMessages         func(config map[string]any, host, expected string) []string
	HookTargetGenerationMessages    func(config map[string]any, host, expected, running string, read func(string) string) []string
	NewInstallPlan                  func(host string, dryRun bool) port.InstallPlan
	PlanHostSkillLinks              func(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error)
	RunningBuildGenerationString    func() string
	SemanticSHA256                  func(value any) (string, error)
	TOMLString                      func(value string) string
	ValidateHookConfigForMerge      func(config map[string]any, knownEvents []string) error
	VerifyHookActivation            func(path string, expected map[string]any) (string, error)
	WriteJSONPlan                   func(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	WriteTextPlan                   func(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error)
}
