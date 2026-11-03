package issueopslease

import (
	"context"
	"fmt"
	"strings"
	"time"

	leasedomain "issueops/internal/domain/issueopslease"
)

const processProbeTimeout = 3 * time.Second

func InspectNativeProcess(ctx context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
	alive, err := pidAlive(receipt.PID)
	if err != nil {
		return "unknown", leasedomain.ProcessReceipt{}, fmt.Errorf("inspect native process identity: %w", err)
	}
	if !alive {
		return "dead", leasedomain.ProcessReceipt{}, nil
	}
	observed, err := observeReceipt(ctx, receipt.PID)
	if err != nil {
		return "unknown", leasedomain.ProcessReceipt{}, fmt.Errorf("inspect live native process identity: %w", err)
	}
	if observed != receipt {
		return "identity-mismatch", observed, nil
	}
	return "live", observed, nil
}

func parseProcessStart(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if started, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return started, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid process start identity %q", value)
}
