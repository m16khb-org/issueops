package lifecycle

import (
	"issueops/internal/adapter/lifecycle/fingerprint"
	lifecyclemodel "issueops/internal/adapter/lifecycle/model"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	lifecycleapp "issueops/internal/application/lifecycle"
	lifecyclecontract "issueops/internal/contract/lifecycle"
)

func testLifecycleService() lifecycleapp.Service {
	return lifecycleapp.Service{SchemaVersion: lifecyclemodel.ProjectLifecycleSchemaVersion, Effects: ProfileFiles{
		Normalize: repopath.NormalizeRoot, StateDir: statestore.StateDir(),
		ObserveFingerprint: func(root string) lifecyclecontract.ProjectFingerprint {
			return fingerprint.ForRoot(root, projectdocs.ReadGitOriginURL)
		},
	}}
}

func ResolveProjectLifecycleState(root string) (ProjectLifecycleStatePlan, error) {
	return testLifecycleService().Resolve(root)
}
func ValidateProjectLifecycleState(root string) (ProjectLifecycleStatePlan, error) {
	return testLifecycleService().Resolve(root)
}
func InitProjectLifecycleState(root string, confirm bool, profiles ...ProjectProfile) (ProjectLifecycleStatePlan, error) {
	return testLifecycleService().Init(root, confirm, profiles...)
}
