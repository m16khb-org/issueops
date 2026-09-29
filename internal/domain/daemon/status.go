package daemon

import (
	contract "issueops/internal/contract/daemon"
	"time"
)

func IdentityMismatchStatus(status contract.Status, message string) contract.Status {
	status.OK = false
	status.IdentityVerified = false
	status.Code = contract.StatusIdentityMismatch
	status.Message = message
	return status
}

func ProcessIdentityMatches(instance contract.InstanceRecord, process contract.ProcessIdentity, location *time.Location) bool {
	if !ProcessStartTimeEqual(instance.ProcessStartTime, process.StartTime, location) {
		return false
	}
	// Linux의 /proc/<pid>/exe는 실행 중인 image를 가리키므로 executable 불일치는
	// 다른 프로세스라는 안정된 증거다. Darwin과 그 밖의 플랫폼에서 ps comm은
	// launcher 경로를 보존하고 EvalSymlinks는 관측 시점의 대상을 해석한다.
	// 따라서 `io update`가 구 daemon 실행 중 symlink를 새 binary로 옮겨도
	// executable projection만 바뀔 수 있다. 호출부의 file/socket handshake가
	// executable, nonce, build, protocol, generation을 봉인하고 start time이
	// signal 직전 PID 재사용을 막는다.
	return !process.ExecutablePathStable || instance.Executable == process.Executable
}

func IsReady(status contract.Status) bool {
	return status.OK && status.Running && status.Reachable && status.IdentityVerified && status.Code == contract.StatusReady && status.Instance != nil && status.PID == status.Instance.PID
}

func BlocksStart(status contract.Status) bool {
	if IsReady(status) {
		return false
	}
	if status.OK && status.Code == contract.StatusStopped && !status.Running && !status.Reachable {
		return false
	}
	return status.PID > 0 || status.Running || status.Reachable || (status.Code != "" && status.Code != contract.StatusStopped)
}

func CanStop(status contract.Status) bool {
	if IsReady(status) {
		return true
	}
	return status.Code == contract.StatusSocketUnreachable &&
		status.Running &&
		!status.Reachable &&
		status.Instance != nil &&
		status.PID > 0 &&
		status.PID == status.Instance.PID
}
