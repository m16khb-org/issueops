package daemoncli

import (
	contract "issueops/internal/contract/daemon"
	"time"
)

const (
	daemonStatusReady              = contract.StatusReady
	daemonStatusStopped            = contract.StatusStopped
	daemonStatusSocketUnreachable  = contract.StatusSocketUnreachable
	daemonStatusIdentityMismatch   = contract.StatusIdentityMismatch
	daemonStatusInstanceUnreadable = contract.StatusInstanceUnreadable
)

const daemonReadyTimeout = 15 * time.Second
