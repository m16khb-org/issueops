package opencode

import (
	"os"

	"issueops/internal/port"
)

// Dependencies are the install-plan and JSON config helpers the opencode
// installer needs. The composition root supplies them so this package does not
// import the shared install machinery.
type Dependencies struct {
	CaptureNativeActivationEvidence func(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error)
	MCPCatalogSHA256                func() (string, error)
	MergeJSONMapFile                func(path string, parent []string, entry string, dryRun bool, value func() (map[string]any, error)) (map[string]any, error)
	NewInstallPlan                  func(host string, dryRun bool) port.InstallPlan
	PlanHostSkillLinks              func(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error)
	RemoveJSONMapEntry              func(path string, parent []string, entry string) (map[string]any, bool, error)
	SemanticSHA256                  func(value any) (string, error)
	VerifyJSONMapEntry              func(path string, parent []string, entry, context string, expected func() (map[string]any, error), digest func(any) (string, error)) (string, error)
	WriteJSONPlan                   func(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	WriteTextPlan                   func(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error)
}
