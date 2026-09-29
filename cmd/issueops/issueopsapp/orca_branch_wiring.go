package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	app "issueops/internal/application/issueopsbranch"
	port "issueops/internal/port/issueopsbranch"
)

func newOrcaBranchPrecheck(root string) app.OrcaBranchPrecheck {
	return app.OrcaBranchPrecheck{Observations: port.OrcaBranchObservations{ReadRecord: (core.CycleRecordStore{StateRoot: root}).Load, RefOID: (core.BranchGit{Run: preflight.GitCmd}).RefOID}}
}
