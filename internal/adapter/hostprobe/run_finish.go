package hostprobe

import (
	"time"

	"issueops/internal/port"
)

// hookEpisodeFinish carries the state a hook-observing host (claude, codex)
// needs after its child process has been started.
type hookEpisodeFinish struct {
	name            string
	request         port.HostProbeRequest
	started         time.Time
	deps            Dependencies
	resultPath      string
	observationPath string
	hookSmoke       bool
}

// finish turns the child process outcome into the final probe result.
func (f hookEpisodeFinish) finish(output CommandOutput, runErr error) port.HostProbeResult {
	fail := func(cause, code string) port.HostProbeResult {
		return failedResult(f.name, "", f.request, f.started, f.deps, cause, code)
	}
	if runErr != nil {
		return fail(normalizedProcessFailure(runErr, "host_process_failed"))
	}
	capture, err := decodeEpisodeCapture(f.resultPath, f.request)
	if err != nil {
		return fail(normalizedCaptureFailure(err))
	}
	result := completedResult(f.name, f.request, f.started, f.deps, capture)
	observation, err := observeHostStream(output.Stdout)
	if err != nil {
		return fail("transport", "host_stream_invalid")
	}
	recorded, err := observeRecordedHookEvents(f.observationPath)
	if err != nil || !recorded.SessionStartObserved || mergeHookObservation(&observation, recorded) != nil {
		return fail("transport", "hook_observation_invalid")
	}
	if !validHostRuntimeObservation(observation) {
		return fail("transport", "host_stream_invalid")
	}
	applyHostStreamObservation(&result, observation, output.ExitCode)
	result.ObservedModel = observation.Model
	result.AmbientToolCount = observation.AmbientToolCount
	if f.hookSmoke {
		if err := persistChildSmokeObservation(f.deps, result, observation.MCPCallCount); err != nil {
			return fail("transport", err.Error())
		}
	}
	return result
}

// omoEpisodeFinish carries the state the omo/omp pi-event-stream hosts need
// after their child process has been started.
type omoEpisodeFinish struct {
	name       string
	request    port.HostProbeRequest
	started    time.Time
	deps       Dependencies
	resultPath string
	targetTool string
}

// finish turns the child process outcome into the final probe result.
func (f omoEpisodeFinish) finish(output CommandOutput, runErr error) port.HostProbeResult {
	fail := func(cause, code string) port.HostProbeResult {
		result := failedResult(f.name, "", f.request, f.started, f.deps, cause, code)
		result.ExitCode = output.ExitCode
		return result
	}
	if runErr != nil {
		return fail(normalizedProcessFailure(runErr, "host_process_failed"))
	}
	if output.ExitCode != 0 {
		return fail("harness_environment", "host_process_failed")
	}
	observation, err := observeOmoStream(output.Stdout, f.targetTool)
	if err != nil {
		cause, code := omoStreamFailure(err)
		result := fail(cause, code)
		result.AmbientToolCount = observation.AmbientToolCount
		result.CallCount = observation.MCPCallCount
		return result
	}
	capture, err := decodeEpisodeCapture(f.resultPath, f.request)
	if err != nil {
		return fail(normalizedCaptureFailure(err))
	}
	if capture.CallCount != 1 || observation.MCPCallCount != 1 {
		return fail("transport", "probe_call_cardinality_invalid")
	}
	result := completedResult(f.name, f.request, f.started, f.deps, capture)
	result.ObservedModel = observation.Model
	// The private guard extension blocks this exact MCP tool unless the managed
	// lifecycle extension injected one hidden project-doc message first.
	result.SessionStartObserved = true
	result.ResponseSHA256 = observation.ResponseSHA256
	result.AmbientToolCount = observation.AmbientToolCount
	result.ExitCode = output.ExitCode
	return result
}
