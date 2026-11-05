package lintdiagnose

import (
	"issueops/internal/adapter/repopath"
	app "issueops/internal/application/lintdiagnose"
	model "issueops/internal/contract/lintdiagnose"
)

func DiagnoseCommand(req model.Request) (model.Result, error) {
	return (app.Service{Effects: Effects{Normalize: repopath.NormalizeRoot}}).Diagnose(req)
}
