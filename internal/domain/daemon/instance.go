package daemon

import (
	"fmt"
	"path/filepath"
	"strings"

	contract "issueops/internal/contract/daemon"
)

func ValidateInstance(r contract.InstanceRecord) error {
	if r.PID <= 0 {
		return fmt.Errorf("pid must be positive")
	}
	for name, value := range map[string]string{
		"process_start_time": r.ProcessStartTime,
		"executable":         r.Executable,
		"instance_nonce":     r.InstanceNonce,
		"build_sha":          r.BuildSHA,
		"protocol_version":   r.ProtocolVersion,
		"generation":         r.Generation,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if !filepath.IsAbs(r.Executable) {
		return fmt.Errorf("executable must be absolute")
	}
	return nil
}
