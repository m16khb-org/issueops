// Package actorreceipt lets record-style commands accept the optional native
// session receipt flags (--session-pid, --session-started-at,
// --session-executable) that `execution whoami` prints in claim_actor_flags,
// so one copied actor vector works for every IssueOps mutation.
//
// The live native process ancestry stays the authority. A supplied receipt
// must name, all three fields together, one entry of that ancestry; otherwise
// the ancestry is withheld and authorization fails closed.
package actorreceipt

import (
	"flag"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

type Flags struct {
	pid        *int
	startedAt  *string
	executable *string
}

// Add registers the optional receipt flags on fs.
func Add(fs *flag.FlagSet) Flags {
	return Flags{
		pid:        fs.Int("session-pid", 0, "optional native session process id (must match the live ancestry)"),
		startedAt:  fs.String("session-started-at", "", "optional native session process start time"),
		executable: fs.String("session-executable", "", "optional native session executable"),
	}
}

// Filter returns ancestry when no receipt was supplied or the supplied receipt
// is one of its entries, and nil otherwise.
func (f Flags) Filter(ancestry []issueopscontract.NativeProcessReceipt) []issueopscontract.NativeProcessReceipt {
	pid, startedAt, executable := *f.pid, strings.TrimSpace(*f.startedAt), strings.TrimSpace(*f.executable)
	if pid == 0 && startedAt == "" && executable == "" {
		return ancestry
	}
	if pid == 0 || startedAt == "" || executable == "" {
		return nil
	}
	for _, receipt := range ancestry {
		if receipt.PID == pid && receipt.StartedAt == startedAt && receipt.Executable == executable {
			return ancestry
		}
	}
	return nil
}

// Observer wraps an ancestry observer with Filter.
func (f Flags) Observer(observe func() ([]issueopscontract.NativeProcessReceipt, error)) func() ([]issueopscontract.NativeProcessReceipt, error) {
	return func() ([]issueopscontract.NativeProcessReceipt, error) {
		ancestry, err := observe()
		if err != nil {
			return nil, err
		}
		return f.Filter(ancestry), nil
	}
}
