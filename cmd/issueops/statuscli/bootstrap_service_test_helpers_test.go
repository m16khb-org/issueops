package statuscli

import (
	lifecyclefiles "issueops/internal/adapter/lifecycle"
	"issueops/internal/adapter/lifecycle/fingerprint"
	lifecyclemodel "issueops/internal/adapter/lifecycle/model"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	lifecycleapp "issueops/internal/application/lifecycle"
	lifecyclecontract "issueops/internal/contract/lifecycle"
)

func testLifecycleService() lifecycleapp.Service {
	return lifecycleapp.Service{SchemaVersion: lifecyclemodel.ProjectLifecycleSchemaVersion, Effects: lifecyclefiles.ProfileFiles{
		Normalize: repopath.NormalizeRoot, StateDir: statestore.StateDir(),
		ObserveFingerprint: func(root string) lifecyclecontract.ProjectFingerprint {
			return fingerprint.ForRoot(root, projectdocs.ReadGitOriginURL)
		},
	}}
}
