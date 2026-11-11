package hostprobe

import "errors"

// Failure sentinels this package returns. The classifiers in omo.go and
// runner.go match them with errors.Is; most map to a result code of the same
// text, read back from Error().
var (
	errCommandCancelled             = errors.New("command_cancelled")
	errCommandFailed                = errors.New("command_failed")
	errCommandOutputTruncated       = errors.New("command_output_truncated")
	errCommandTerminationFailed     = errors.New("command_termination_failed")
	errCommandTimeout               = errors.New("command_timeout")
	errHostStreamInvalid            = errors.New("host_stream_invalid")
	errHostStreamModelMissing       = errors.New("host_stream_model_missing")
	errHostStreamMultipleCalls      = errors.New("host_stream_multiple_calls")
	errHostStreamNoCall             = errors.New("host_stream_no_call")
	errHostStreamNonTargetCall      = errors.New("host_stream_non_target_call")
	errHostStreamToolError          = errors.New("host_stream_tool_error")
	errMultipleCallMarkerInvalid    = errors.New("multiple_call_marker_invalid")
	errMultipleCallMarkerReadFailed = errors.New("multiple_call_marker_read_failed")
	errMultipleCallMarkerTooLarge   = errors.New("multiple_call_marker_too_large")
	errProbeResultIdentityMismatch  = errors.New("probe_result_identity_mismatch")
	errProbeResultInvalid           = errors.New("probe_result_invalid")
	errProbeResultMissing           = errors.New("probe_result_missing")
	errProbeResultReadFailed        = errors.New("probe_result_read_failed")
	errProbeResultTooLarge          = errors.New("probe_result_too_large")
	errStaleRunToken                = errors.New("stale_run_token")
)

// matchedFailureCode returns the text of the first sentinel that err wraps.
func matchedFailureCode(err error, sentinels ...error) (string, bool) {
	for _, sentinel := range sentinels {
		if errors.Is(err, sentinel) {
			return sentinel.Error(), true
		}
	}
	return "", false
}
