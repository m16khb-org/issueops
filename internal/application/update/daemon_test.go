package update

import (
	"errors"
	contract "issueops/internal/contract/update"
	"reflect"
	"testing"
)

func TestStaleDaemonsStopsAtFirstFailedSignal(t *testing.T) {
	failure := errors.New("signal rejected")
	var signaled []int
	service := StaleDaemons{
		List: func() ([]contract.DaemonProcess, error) {
			return []contract.DaemonProcess{{PID: 10}, {PID: 11}, {PID: 12}, {PID: 13}}, nil
		},
		CurrentPID: func() int { return 10 },
		Terminate: func(pid int) error {
			signaled = append(signaled, pid)
			if pid == 12 {
				return failure
			}
			return nil
		},
	}
	count, err := service.Run()
	if !errors.Is(err, failure) || count != 1 || !reflect.DeepEqual(signaled, []int{11, 12}) {
		t.Fatalf("partial cleanup: count=%d signals=%v err=%v", count, signaled, err)
	}
}
func TestDaemonRefreshPropagatesInventoryFailureAfterStop(t *testing.T) {
	failure := errors.New("inventory failed")
	var calls []string
	service := DaemonRefresh{
		Stop:    func() error { calls = append(calls, "stop"); return nil },
		Cleanup: StaleDaemons{List: func() ([]contract.DaemonProcess, error) { calls = append(calls, "list"); return nil, failure }, CurrentPID: func() int { t.Fatal("failed list must not inspect current process"); return 10 }, Terminate: func(int) error { t.Fatal("failed list must not signal"); return nil }},
	}
	if err := service.Run(); !errors.Is(err, failure) || !reflect.DeepEqual(calls, []string{"stop", "list"}) {
		t.Fatalf("refresh: calls=%v err=%v", calls, err)
	}
}
