package lifecycle

import (
	"issueops/internal/adapter/lifecycle/model"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

const ProjectLifecycleSchemaVersion = model.ProjectLifecycleSchemaVersion

type ProjectProfile = projectdoccontract.ProjectProfile
type ProjectLifecycleProfile = lifecyclecontract.ProjectLifecycleProfile
type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan
