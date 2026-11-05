package issueopslease

import (
	"context"
	"fmt"
	"strings"
	"time"

	leasedomain "issueops/internal/domain/issueopslease"
)

const processProbeTimeout = 3 * time.Second

// InspectNativeProcess 함수는 receipt가 가리키는 프로세스의 현재 상태를 조사해 "live", "dead", "identity-mismatch", "unknown" 중 하나와 관측한 영수증을 반환한다.
// PID가 살아 있어도 시작 시각이나 실행 파일 경로가 receipt와 다르면 PID가 재사용된 것으로 보고 "identity-mismatch"를 반환한다.
// 조사 자체가 실패하면 "unknown"과 오류를 반환한다.
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
