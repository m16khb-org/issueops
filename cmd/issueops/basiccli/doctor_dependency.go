package basiccli

import (
	"context"
	"time"

	doctorapp "issueops/internal/application/doctor"
	"issueops/internal/domain/operationalhealth"
)

// Doctor captures the diagnostic application and host observations for one caller.
type Doctor struct {
	Service                     doctorapp.Service
	NormalizeRepoRoot           func(string) (string, error)
	IssueOpsRoot, Home, Version string
	Now                         func() time.Time
	CollectOperationalHealth    func(context.Context, string) operationalhealth.Snapshot
}
