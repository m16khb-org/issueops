package toolconformance

import (
	"context"
	"testing"
)

func TestRunLiveBenchmarkRejectsInvalidRequestBeforeEffects(t *testing.T) {
	result, err := RunLiveBenchmark(context.Background(), LiveBenchmarkRequest{}, nil, LiveBenchmarkDependencies{})
	if err == nil || err.Error() != "hosts_required" || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
