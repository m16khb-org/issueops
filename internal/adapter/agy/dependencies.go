package agy

import (
	"issueops/internal/port"
	"os"
)

// Dependencies contains the file and host capabilities used by one installer.
type Dependencies struct {
	CaptureNativeActivationEvidence func(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error)
	MCPCatalogSHA256                func() (string, error)
	NewInstallPlan                  func(host string, dryRun bool) port.InstallPlan
	PlanHostSkillLinks              func(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error)
	SemanticSHA256                  func(value any) (string, error)
	WriteJSONPlan                   func(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error)
}
