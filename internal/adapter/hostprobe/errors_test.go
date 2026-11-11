package hostprobe

import (
	"fmt"
	"testing"
)

var probeFailureCodes = []struct {
	err  error
	text string
}{
	{errCommandCancelled, "command_cancelled"},
	{errCommandFailed, "command_failed"},
	{errCommandOutputTruncated, "command_output_truncated"},
	{errCommandTerminationFailed, "command_termination_failed"},
	{errCommandTimeout, "command_timeout"},
	{errHostStreamInvalid, "host_stream_invalid"},
	{errHostStreamModelMissing, "host_stream_model_missing"},
	{errHostStreamMultipleCalls, "host_stream_multiple_calls"},
	{errHostStreamNoCall, "host_stream_no_call"},
	{errHostStreamNonTargetCall, "host_stream_non_target_call"},
	{errHostStreamToolError, "host_stream_tool_error"},
	{errMultipleCallMarkerInvalid, "multiple_call_marker_invalid"},
	{errMultipleCallMarkerReadFailed, "multiple_call_marker_read_failed"},
	{errMultipleCallMarkerTooLarge, "multiple_call_marker_too_large"},
	{errProbeResultIdentityMismatch, "probe_result_identity_mismatch"},
	{errProbeResultInvalid, "probe_result_invalid"},
	{errProbeResultMissing, "probe_result_missing"},
	{errProbeResultReadFailed, "probe_result_read_failed"},
	{errProbeResultTooLarge, "probe_result_too_large"},
	{errStaleRunToken, "stale_run_token"},
}

func TestProbeFailureCodesKeepTheirText(t *testing.T) {
	if len(probeFailureCodes) != 20 {
		t.Fatalf("table has %d codes, want 20", len(probeFailureCodes))
	}
	for _, tc := range probeFailureCodes {
		if got := tc.err.Error(); got != tc.text {
			t.Errorf("%v text = %q, want %q", tc.err, got, tc.text)
		}
	}
}

func TestProbeFailureCodesSurviveWrapping(t *testing.T) {
	classifiers := []struct {
		name string
		run  func(error) (string, string)
	}{
		{"omoStreamFailure", omoStreamFailure},
		{"normalizedProcessFailure", func(err error) (string, string) { return normalizedProcessFailure(err, "dflt") }},
		{"normalizedCaptureFailure", normalizedCaptureFailure},
	}
	for _, tc := range probeFailureCodes {
		wrapped := fmt.Errorf("observe: %w", tc.err)
		for _, classifier := range classifiers {
			wantCause, wantCode := classifier.run(tc.err)
			gotCause, gotCode := classifier.run(wrapped)
			if gotCause != wantCause || gotCode != wantCode {
				t.Errorf("%s(wrapped %v) = %q/%q, want %q/%q", classifier.name, tc.err, gotCause, gotCode, wantCause, wantCode)
			}
		}
	}
	// Spot-check that wrapping does not fall through to the default branches.
	if cause, code := normalizedProcessFailure(fmt.Errorf("run: %w", errCommandTimeout), "dflt"); cause != "harness_environment" || code != "command_timeout" {
		t.Errorf("wrapped timeout = %q/%q", cause, code)
	}
	if cause, code := omoStreamFailure(fmt.Errorf("observe: %w", errHostStreamNoCall)); cause != "unknown" || code != "no_call" {
		t.Errorf("wrapped no call = %q/%q", cause, code)
	}
	if cause, code := normalizedCaptureFailure(fmt.Errorf("capture: %w", errProbeResultTooLarge)); cause != "transport" || code != "probe_result_too_large" {
		t.Errorf("wrapped too large = %q/%q", cause, code)
	}
}
