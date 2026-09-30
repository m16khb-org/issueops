package lintdiagnose

import (
	"issueops/internal/adapter/repopath"
	app "issueops/internal/application/lintdiagnose"
	model "issueops/internal/contract/lintdiagnose"
)

func DiagnoseCommand(req model.LintDiagnoseRequest) (model.LintDiagnoseResult, error) {
	return (app.Service{Effects: Effects{Normalize: repopath.NormalizeRoot}}).Diagnose(req)
}
