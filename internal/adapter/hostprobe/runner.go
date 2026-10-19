package hostprobe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	toolconformancecontract "issueops/internal/contract/toolconformance"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

const (
	MaxOutputBytes              = 64 << 10
	EpisodeTimeout              = 5 * time.Minute
	VersionTimeout              = 10 * time.Second
	processTerminationGrace     = 250 * time.Millisecond
	processTerminationHardLimit = 2 * time.Second
)

var errPrivateRootCleanup = errors.New("private_root_cleanup_failed")

type CommandRequest struct {
	Cwd      string
	Argv     []string
	Env      []string
	Timeout  time.Duration
	OmoJSONL bool
	// Stdin, when set, is the child's standard input.
	Stdin string
}

type CommandOutput struct {
	Stdout          []byte
	Stderr          []byte
	ExitCode        int
	StdoutTruncated bool
	StderrTruncated bool
}

type CommandRunner interface {
	Run(context.Context, CommandRequest) (CommandOutput, error)
}

type Dependencies struct {
	Process   CommandRunner
	LookPath  func(string) (string, error)
	Now       func() time.Time
	TempDir   func(string, string) (string, error)
	RemoveAll func(string) error
	Getenv    func(string) string
	Environ   func() []string
	// StartServer and HTTPGet drive a host's private background server.
	StartServer func(context.Context, ServerRequest) (ServerProcess, error)
	HTTPGet     func(ctx context.Context, url, user, password string) (int, []byte, error)
}

type ExecRunner struct{}

type boundedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || written > 0
		return written, nil
	}
	if len(value) > remaining {
		value = value[:remaining]
		b.truncated = true
	}
	_, err := b.Buffer.Write(value)
	return written, err
}

func (ExecRunner) Run(ctx context.Context, request CommandRequest) (CommandOutput, error) {
	if len(request.Argv) == 0 {
		return CommandOutput{}, fmt.Errorf("empty_argv")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = EpisodeTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
	configureProcessGroup(cmd)
	cmd.Dir = request.Cwd
	if request.Env != nil {
		cmd.Env = append([]string(nil), request.Env...)
	}
	if request.Stdin != "" {
		cmd.Stdin = strings.NewReader(request.Stdin)
	}
	stdout := &boundedBuffer{limit: MaxOutputBytes}
	stderr := &boundedBuffer{limit: MaxOutputBytes}
	// Hide bytes.Buffer.ReadFrom: io.Copy must use the bounded Write method.
	cmd.Stdout = struct{ io.Writer }{stdout}
	cmd.Stderr = struct{ io.Writer }{stderr}
	var projection *omoOutput
	if request.OmoJSONL {
		projection = &omoOutput{output: stdout}
		cmd.Stdout = projection
	}
	if err := cmd.Start(); err != nil {
		return CommandOutput{}, fmt.Errorf("command_failed")
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	err, terminationErr := awaitCommand(runCtx, cmd, waited)
	if terminationErr != nil {
		return CommandOutput{}, terminationErr
	}
	if projection != nil {
		projection.flush()
	}
	out := CommandOutput{
		Stdout:          append([]byte(nil), stdout.Bytes()...),
		Stderr:          append([]byte(nil), stderr.Bytes()...),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}
	if cmd.ProcessState != nil {
		out.ExitCode = cmd.ProcessState.ExitCode()
	}
	return out, commandOutcomeError(runCtx, out, err)
}

// awaitCommand returns the command's wait error, or a termination error when
// the process tree could not be reaped after the run context ended.
func awaitCommand(runCtx context.Context, cmd *exec.Cmd, waited <-chan error) (error, error) {
	select {
	case err := <-waited:
		return err, nil
	case <-runCtx.Done():
	}
	if terminateProcessTree(cmd) != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case err := <-waited:
		return err, nil
	case <-time.After(processTerminationGrace):
	}
	_ = cmd.Process.Kill()
	select {
	case err := <-waited:
		return err, nil
	case <-time.After(processTerminationHardLimit):
		return nil, fmt.Errorf("command_termination_failed")
	}
}

func commandOutcomeError(runCtx context.Context, out CommandOutput, waitErr error) error {
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("command_timeout")
	}
	if runCtx.Err() != nil {
		return fmt.Errorf("command_cancelled")
	}
	if waitErr != nil {
		return fmt.Errorf("command_failed")
	}
	if out.StdoutTruncated || out.StderrTruncated {
		return fmt.Errorf("command_output_truncated")
	}
	return nil
}

func normalizeDependencies(deps Dependencies) Dependencies {
	if deps.Process == nil {
		deps.Process = ExecRunner{}
	}
	if deps.LookPath == nil {
		deps.LookPath = exec.LookPath
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.TempDir == nil {
		deps.TempDir = os.MkdirTemp
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.Getenv == nil {
		deps.Getenv = os.Getenv
	}
	if deps.Environ == nil {
		deps.Environ = os.Environ
	}
	if deps.StartServer == nil {
		deps.StartServer = startExecServer
	}
	if deps.HTTPGet == nil {
		deps.HTTPGet = basicAuthGet
	}
	return deps
}

func preflight(ctx context.Context, deps Dependencies, host, executable string, request port.HostProbeRequest, env []string) port.HostProbePreflight {
	result := port.HostProbePreflight{Host: host, RequestedModel: request.Model}
	path, err := deps.LookPath(executable)
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "executable_not_found"
		result.EvidenceSource = host + "_preflight"
		return result
	}
	result.Installed = true
	output, err := deps.Process.Run(ctx, CommandRequest{Argv: []string{path, "--version"}, Env: env, Timeout: VersionTimeout})
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "version_probe_failed"
		result.EvidenceSource = host + "_preflight"
		return result
	}
	result.Ready = true
	result.Version = boundedVersion(string(output.Stdout))
	result.EvidenceSource = host + "_preflight"
	return result
}

func boundedVersion(value string) string {
	value = strings.TrimSpace(policy.RedactFreeform(value))
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	if len(value) > 256 {
		value = policy.TruncateBytes(value, 256)
	}
	return value
}

type episodeCapture struct {
	FixtureID          string                                 `json:"fixture_id"`
	CallCount          int                                    `json:"call_count"`
	RawSHA256          string                                 `json:"raw_sha256"`
	CanonicalArguments any                                    `json:"canonical_arguments"`
	SchemaSHA256       string                                 `json:"schema_sha256"`
	RunTokenSHA256     string                                 `json:"run_token_sha256"`
	Classification     toolconformancecontract.Classification `json:"classification"`
	AdvertisedValid    bool                                   `json:"advertised_valid"`
	CanonicalValid     bool                                   `json:"canonical_valid"`
	Diagnostics        []toolconformancecontract.Diagnostic   `json:"diagnostics"`
}

func newEpisodeRoot(deps Dependencies, host string) (string, error) {
	root, err := deps.TempDir("", "issueops-conformance-"+host+"-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		if cleanupErr := deps.RemoveAll(root); cleanupErr != nil {
			return "", errPrivateRootCleanup
		}
		return "", err
	}
	return root, nil
}

func episodeRootFailureCode(err error) string {
	if errors.Is(err, errPrivateRootCleanup) {
		return "private_root_cleanup_failed"
	}
	return "episode_root_create_failed"
}

func serveArgv(request port.HostProbeRequest, resultPath string) []string {
	return []string{
		request.HarnessBinary,
		"contract", "conformance", "serve",
		"--fixture-id", request.FixtureID,
		"--result-file", resultPath,
		"--run-token", request.RunToken,
	}
}

func decodeEpisodeCapture(resultPath string, request port.HostProbeRequest) (episodeCapture, error) {
	capture, err := readEpisodeCapture(resultPath)
	if err != nil {
		return episodeCapture{}, err
	}
	if err := validateEpisodeCapture(capture, request); err != nil {
		return episodeCapture{}, err
	}
	if err := applyMultipleCallMarker(resultPath, &capture); err != nil {
		return episodeCapture{}, err
	}
	return capture, nil
}

func readEpisodeCapture(resultPath string) (episodeCapture, error) {
	data, err := readBoundedEvidenceFile(resultPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return episodeCapture{}, fmt.Errorf("probe_result_missing")
		}
		if err.Error() == "evidence_file_too_large" {
			return episodeCapture{}, fmt.Errorf("probe_result_too_large")
		}
		return episodeCapture{}, fmt.Errorf("probe_result_read_failed")
	}
	var capture episodeCapture
	if err := decodeStrictJSON(data, &capture); err != nil {
		return episodeCapture{}, fmt.Errorf("probe_result_invalid")
	}
	return capture, nil
}

func validateEpisodeCapture(capture episodeCapture, request port.HostProbeRequest) error {
	want := sha256.Sum256([]byte(request.RunToken))
	wantToken := hex.EncodeToString(want[:])
	if capture.RunTokenSHA256 != wantToken {
		return fmt.Errorf("stale_run_token")
	}
	if capture.FixtureID != request.FixtureID || capture.SchemaSHA256 != request.SchemaSHA256 {
		return fmt.Errorf("probe_result_identity_mismatch")
	}
	if capture.CallCount != 1 || capture.CanonicalArguments == nil || !validSHA256(capture.RawSHA256) || !validSHA256(capture.RunTokenSHA256) {
		return fmt.Errorf("probe_result_invalid")
	}
	if _, err := toolconformancecontract.ParseClassification(string(capture.Classification)); err != nil {
		return fmt.Errorf("probe_result_invalid")
	}
	return nil
}

func applyMultipleCallMarker(resultPath string, capture *episodeCapture) error {
	multiplePath := resultPath + ".multiple"
	multipleData, markerErr := readBoundedEvidenceFile(multiplePath)
	if markerErr == nil {
		var marker struct {
			CallCount      int    `json:"call_count"`
			RunTokenSHA256 string `json:"run_token_sha256"`
		}
		if decodeStrictJSON(multipleData, &marker) != nil || marker.RunTokenSHA256 != capture.RunTokenSHA256 || marker.CallCount <= capture.CallCount {
			return fmt.Errorf("multiple_call_marker_invalid")
		}
		capture.CallCount = marker.CallCount
		capture.Classification = toolconformancecontract.Classification(toolconformancecontract.MultipleCalls)
		capture.AdvertisedValid = false
		capture.CanonicalValid = false
		capture.Diagnostics = []toolconformancecontract.Diagnostic{}
	} else if !errors.Is(markerErr, fs.ErrNotExist) {
		if markerErr.Error() == "evidence_file_too_large" {
			return fmt.Errorf("multiple_call_marker_too_large")
		}
		return fmt.Errorf("multiple_call_marker_read_failed")
	}
	return nil
}

func readBoundedEvidenceFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxOutputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxOutputBytes {
		return nil, fmt.Errorf("evidence_file_too_large")
	}
	return data, nil
}

func decodeStrictJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing_json")
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func completedResult(host, version string, request port.HostProbeRequest, started time.Time, deps Dependencies, capture episodeCapture) port.HostProbeResult {
	if version == "" {
		version = request.HostVersion
	}
	arguments, _ := json.Marshal(capture.CanonicalArguments)
	diagnostics, _ := json.Marshal(capture.Diagnostics)
	return port.HostProbeResult{
		Completed:              true,
		Host:                   host,
		HostVersion:            version,
		RequestedModel:         request.Model,
		FixtureID:              request.FixtureID,
		SchemaSHA256:           capture.SchemaSHA256,
		Profile:                request.Profile,
		Attempt:                request.Attempt,
		DurationMS:             deps.Now().Sub(started).Milliseconds(),
		AmbientToolCount:       1,
		CallCount:              capture.CallCount,
		RawArgumentsSHA256:     capture.RawSHA256,
		CanonicalArgumentsJSON: string(arguments),
		EvidenceID:             capture.RunTokenSHA256,
		Classification:         string(capture.Classification),
		AdvertisedValid:        capture.AdvertisedValid,
		CanonicalValid:         capture.CanonicalValid,
		DiagnosticsJSON:        string(diagnostics),
	}
}

func applyHostStreamObservation(result *port.HostProbeResult, observation hostStreamObservation, exitCode int) {
	result.SessionStartObserved = observation.SessionStartObserved
	result.PreToolUseObserved = observation.PreToolUseObserved
	result.ResponseSHA256 = observation.ResponseSHA256
	result.ExitCode = exitCode
}

func persistChildSmokeObservation(deps Dependencies, result port.HostProbeResult, mcpCallCount int) error {
	path := strings.TrimSpace(deps.Getenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"))
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("child_smoke_observation_path_invalid")
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || parentInfo.Mode().Perm() != 0o700 {
		return fmt.Errorf("child_smoke_observation_path_invalid")
	}
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("child_smoke_observation_path_invalid")
	}
	observation := hostStreamObservation{
		SessionStartObserved: result.SessionStartObserved,
		PreToolUseObserved:   result.PreToolUseObserved,
		MCPCallCount:         mcpCallCount,
		ResponseSHA256:       result.ResponseSHA256,
		ExitCode:             result.ExitCode,
		DurationMS:           result.DurationMS,
	}
	data, err := json.Marshal(observation)
	if err != nil {
		return fmt.Errorf("child_smoke_observation_encode_failed")
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	return nil
}

func failedResult(host, version string, request port.HostProbeRequest, started time.Time, deps Dependencies, cause, code string) port.HostProbeResult {
	if version == "" {
		version = request.HostVersion
	}
	return port.HostProbeResult{
		Host:             host,
		HostVersion:      version,
		RequestedModel:   request.Model,
		FixtureID:        request.FixtureID,
		SchemaSHA256:     request.SchemaSHA256,
		Profile:          request.Profile,
		Attempt:          request.Attempt,
		DurationMS:       deps.Now().Sub(started).Milliseconds(),
		Cause:            cause,
		Code:             code,
		EvidenceSource:   host + "_runner",
		AmbientToolCount: 1,
	}
}

func writePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func normalizedProcessFailure(err error, defaultCode string) (string, string) {
	switch err.Error() {
	case "command_timeout", "command_cancelled":
		return "harness_environment", err.Error()
	case "command_output_truncated":
		return "transport", "command_output_truncated"
	default:
		return "harness_environment", defaultCode
	}
}

func normalizedCaptureFailure(err error) (string, string) {
	switch err.Error() {
	case "stale_run_token":
		return "harness_environment", "stale_run_token"
	case "probe_result_missing":
		return "unknown", "no_call"
	case "probe_result_read_failed", "probe_result_invalid", "probe_result_too_large", "probe_result_identity_mismatch",
		"multiple_call_marker_invalid", "multiple_call_marker_too_large", "multiple_call_marker_read_failed":
		return "transport", err.Error()
	default:
		return "transport", "probe_result_decode_failed"
	}
}
func isolatedHostEnv(deps Dependencies, additionalNames ...string) []string {
	allowed := map[string]bool{}
	for _, name := range additionalNames {
		allowed[name] = true
	}
	values := map[string]string{}
	for _, entry := range deps.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if found && (isHostAmbientEnvironment(name) || allowed[name]) {
			values[name] = value
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+"="+values[name])
	}
	return out
}

func isHostAmbientEnvironment(name string) bool {
	switch name {
	case "PATH", "HOME", "USER", "TMPDIR", "LANG", "LANGUAGE", "LC_ALL", "LC_ADDRESS", "LC_COLLATE", "LC_CTYPE", "LC_IDENTIFICATION", "LC_MEASUREMENT", "LC_MESSAGES", "LC_MONETARY", "LC_NAME", "LC_NUMERIC", "LC_PAPER", "LC_TELEPHONE", "LC_TIME":
		return true
	default:
		return false
	}
}
