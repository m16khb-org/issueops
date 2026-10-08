package issueops

import (
	"context"
	"testing"
	"time"
)

// sleepWithContext는 await-link 폴링의 취소 가능 대기다. 타이밍 자체가
// 동작이므로 컨텍스트 취소와 정상 완료 두 경계만 잠근다.
func TestSleepWithContextCancellationAndCompletion(t *testing.T) {
	t.Parallel()

	if err := SleepWithContext(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("elapsed sleep must complete: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SleepWithContext(ctx, time.Hour); err == nil {
		t.Fatal("cancelled context must return immediately")
	}
}
