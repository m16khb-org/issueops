package hostprobe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/port"
)

func captureRequest() port.HostProbeRequest {
	return port.HostProbeRequest{RunToken: "token-1", FixtureID: "fx", SchemaSHA256: strings.Repeat("a", 64)}
}

func captureJSON(t *testing.T, request port.HostProbeRequest, mutate func(map[string]any)) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(request.RunToken))
	doc := map[string]any{
		"fixture_id":          request.FixtureID,
		"call_count":          1,
		"raw_sha256":          strings.Repeat("b", 64),
		"canonical_arguments": map[string]any{"k": "v"},
		"schema_sha256":       request.SchemaSHA256,
		"run_token_sha256":    hex.EncodeToString(sum[:]),
		"classification":      "exact_valid",
		"advertised_valid":    true,
		"canonical_valid":     true,
		"diagnostics":         []any{},
	}
	if mutate != nil {
		mutate(doc)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeEpisodeCaptureRejectionsMapToNormalizedFailures(t *testing.T) {
	request := captureRequest()
	cases := []struct {
		name      string
		data      []byte // nil: no file
		wantErr   string
		wantCause string
		wantCode  string
	}{
		{"missing file", nil, "probe_result_missing", "unknown", "no_call"},
		{"stale token", captureJSON(t, request, func(m map[string]any) { m["run_token_sha256"] = strings.Repeat("c", 64) }), "stale_run_token", "harness_environment", "stale_run_token"},
		{"fixture mismatch", captureJSON(t, request, func(m map[string]any) { m["fixture_id"] = "other" }), "probe_result_identity_mismatch", "transport", "probe_result_identity_mismatch"},
		{"schema mismatch", captureJSON(t, request, func(m map[string]any) { m["schema_sha256"] = strings.Repeat("d", 64) }), "probe_result_identity_mismatch", "transport", "probe_result_identity_mismatch"},
		{"zero calls", captureJSON(t, request, func(m map[string]any) { m["call_count"] = 0 }), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"null arguments", captureJSON(t, request, func(m map[string]any) { m["canonical_arguments"] = nil }), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"short raw digest", captureJSON(t, request, func(m map[string]any) { m["raw_sha256"] = "abcd" }), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"non hex raw digest", captureJSON(t, request, func(m map[string]any) { m["raw_sha256"] = strings.Repeat("z", 64) }), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"unknown classification", captureJSON(t, request, func(m map[string]any) { m["classification"] = "looks_fine" }), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"unknown field", captureJSON(t, request, func(m map[string]any) { m["extra"] = 1 }), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"trailing json", append(captureJSON(t, request, nil), []byte(` {}`)...), "probe_result_invalid", "transport", "probe_result_invalid"},
		{"oversize", []byte(strings.Repeat(" ", MaxOutputBytes+1)), "probe_result_too_large", "transport", "probe_result_too_large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")
			if tc.data != nil {
				if err := os.WriteFile(path, tc.data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := decodeEpisodeCapture(path, request)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			cause, code := normalizedCaptureFailure(err)
			if cause != tc.wantCause || code != tc.wantCode {
				t.Fatalf("normalized = %q/%q, want %q/%q", cause, code, tc.wantCause, tc.wantCode)
			}
		})
	}
}

func TestDecodeEpisodeCaptureUnreadableResultIsReadFailure(t *testing.T) {
	dir := t.TempDir() // a directory in place of the result file cannot be read
	_, err := decodeEpisodeCapture(dir, captureRequest())
	if err == nil || err.Error() != "probe_result_read_failed" {
		t.Fatalf("err = %v", err)
	}
	if cause, code := normalizedCaptureFailure(err); cause != "transport" || code != "probe_result_read_failed" {
		t.Fatalf("normalized = %q/%q", cause, code)
	}
}

func TestNormalizedCaptureFailureUnknownErrorIsDecodeFailure(t *testing.T) {
	cause, code := normalizedCaptureFailure(errors.New("something else"))
	if cause != "transport" || code != "probe_result_decode_failed" {
		t.Fatalf("normalized = %q/%q", cause, code)
	}
}

func TestDecodeEpisodeCaptureAcceptsValidCapture(t *testing.T) {
	request := captureRequest()
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, captureJSON(t, request, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	capture, err := decodeEpisodeCapture(path, request)
	if err != nil {
		t.Fatal(err)
	}
	if capture.CallCount != 1 || capture.Classification != "exact_valid" || !capture.AdvertisedValid || !capture.CanonicalValid {
		t.Fatalf("capture = %+v", capture)
	}
}

func TestMultipleCallMarkerOverridesCaptureOutcome(t *testing.T) {
	request := captureRequest()
	sum := sha256.Sum256([]byte(request.RunToken))
	token := hex.EncodeToString(sum[:])
	write := func(t *testing.T, marker string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "result.json")
		if err := os.WriteFile(path, captureJSON(t, request, nil), 0o600); err != nil {
			t.Fatal(err)
		}
		if marker != "" {
			if err := os.WriteFile(path+".multiple", []byte(marker), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return path
	}
	t.Run("valid marker downgrades to multiple_calls", func(t *testing.T) {
		path := write(t, `{"call_count":3,"run_token_sha256":"`+token+`"}`)
		capture, err := decodeEpisodeCapture(path, request)
		if err != nil {
			t.Fatal(err)
		}
		if capture.CallCount != 3 || capture.Classification != "multiple_calls" || capture.AdvertisedValid || capture.CanonicalValid || capture.Diagnostics == nil || len(capture.Diagnostics) != 0 {
			t.Fatalf("capture = %+v", capture)
		}
	})
	rejected := map[string]string{
		"token mismatch":        `{"call_count":3,"run_token_sha256":"` + strings.Repeat("e", 64) + `"}`,
		"not more than capture": `{"call_count":1,"run_token_sha256":"` + token + `"}`,
		"unknown field":         `{"call_count":3,"run_token_sha256":"` + token + `","x":1}`,
		"malformed":             `{`,
	}
	for name, marker := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := decodeEpisodeCapture(write(t, marker), request)
			if err == nil || err.Error() != "multiple_call_marker_invalid" {
				t.Fatalf("err = %v", err)
			}
			if cause, code := normalizedCaptureFailure(err); cause != "transport" || code != "multiple_call_marker_invalid" {
				t.Fatalf("normalized = %q/%q", cause, code)
			}
		})
	}
	t.Run("oversize marker", func(t *testing.T) {
		_, err := decodeEpisodeCapture(write(t, strings.Repeat(" ", MaxOutputBytes+1)), request)
		if err == nil || err.Error() != "multiple_call_marker_too_large" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unreadable marker", func(t *testing.T) {
		path := write(t, "")
		if err := os.Mkdir(path+".multiple", 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := decodeEpisodeCapture(path, request)
		if err == nil || err.Error() != "multiple_call_marker_read_failed" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestNormalizedProcessFailure(t *testing.T) {
	cases := []struct{ in, cause, code string }{
		{"command_timeout", "harness_environment", "command_timeout"},
		{"command_cancelled", "harness_environment", "command_cancelled"},
		{"command_output_truncated", "transport", "command_output_truncated"},
		{"command_failed", "harness_environment", "dflt"},
	}
	for _, tc := range cases {
		cause, code := normalizedProcessFailure(errors.New(tc.in), "dflt")
		if cause != tc.cause || code != tc.code {
			t.Errorf("%s -> %q/%q, want %q/%q", tc.in, cause, code, tc.cause, tc.code)
		}
	}
}

func TestCommandOutcomeErrorPrecedence(t *testing.T) {
	live := context.Background()
	timedOut, cancelT := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelT()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	truncated := CommandOutput{StdoutTruncated: true}
	cases := []struct {
		name    string
		ctx     context.Context
		out     CommandOutput
		waitErr error
		want    string
	}{
		{"deadline beats wait error", timedOut, CommandOutput{}, errors.New("x"), "command_timeout"},
		{"cancel beats wait error", cancelled, CommandOutput{}, errors.New("x"), "command_cancelled"},
		{"wait error beats truncation", live, truncated, errors.New("x"), "command_failed"},
		{"stdout truncation", live, truncated, nil, "command_output_truncated"},
		{"stderr truncation", live, CommandOutput{StderrTruncated: true}, nil, "command_output_truncated"},
	}
	for _, tc := range cases {
		err := commandOutcomeError(tc.ctx, tc.out, tc.waitErr)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.want)
		}
	}
	if err := commandOutcomeError(live, CommandOutput{}, nil); err != nil {
		t.Fatalf("clean outcome err = %v", err)
	}
}

func TestExecRunnerReportsStartExitAndCancellationFailures(t *testing.T) {
	ctx := t.Context()
	if _, err := (ExecRunner{}).Run(ctx, CommandRequest{}); err == nil || err.Error() != "empty_argv" {
		t.Fatalf("empty argv err = %v", err)
	}
	if _, err := (ExecRunner{}).Run(ctx, CommandRequest{Argv: []string{filepath.Join(t.TempDir(), "nope")}}); err == nil || err.Error() != "command_failed" {
		t.Fatalf("start failure err = %v", err)
	}
	out, err := (ExecRunner{}).Run(ctx, CommandRequest{Argv: []string{"/bin/sh", "-c", "echo hi; exit 3"}, Env: []string{}})
	if err == nil || err.Error() != "command_failed" || out.ExitCode != 3 || strings.TrimSpace(string(out.Stdout)) != "hi" {
		t.Fatalf("exit failure: out=%+v err=%v", out, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := (ExecRunner{}).Run(cancelled, CommandRequest{Argv: []string{"/bin/sh", "-c", "sleep 30"}}); err == nil || err.Error() != "command_cancelled" {
		t.Fatalf("cancel err = %v", err)
	}
}

func TestPersistChildSmokeObservation(t *testing.T) {
	result := port.HostProbeResult{SessionStartObserved: true, PreToolUseObserved: false, ResponseSHA256: strings.Repeat("f", 64), ExitCode: 7, DurationMS: 42}
	depsFor := func(path string) Dependencies {
		return Dependencies{Getenv: func(name string) string {
			if name != "ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE" {
				t.Errorf("unexpected env lookup %q", name)
			}
			return path
		}}
	}
	privateDir := func(t *testing.T, mode os.FileMode) string {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "obs")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	t.Run("unset is a no-op", func(t *testing.T) {
		if err := persistChildSmokeObservation(depsFor("  "), result, 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("writes a private single-line observation", func(t *testing.T) {
		path := filepath.Join(privateDir(t, 0o700), "o.json")
		if err := persistChildSmokeObservation(depsFor(path), result, 1); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("stat = %v, %v", info, err)
		}
		data, _ := os.ReadFile(path)
		var got hostStreamObservation
		if err := decodeStrictJSON(data, &got); err != nil {
			t.Fatalf("decode %q: %v", data, err)
		}
		if !got.SessionStartObserved || got.PreToolUseObserved || got.MCPCallCount != 1 || got.ResponseSHA256 != result.ResponseSHA256 || got.ExitCode != 7 || got.DurationMS != 42 || !strings.HasSuffix(string(data), "\n") {
			t.Fatalf("observation = %+v (%q)", got, data)
		}
	})
	rejections := map[string]func(t *testing.T) string{
		"relative path":         func(t *testing.T) string { return "rel/o.json" },
		"missing parent":        func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent", "o.json") },
		"group-readable parent": func(t *testing.T) string { return filepath.Join(privateDir(t, 0o750), "o.json") },
		"existing file": func(t *testing.T) string {
			path := filepath.Join(privateDir(t, 0o700), "o.json")
			if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		},
		"dangling symlink target": func(t *testing.T) string {
			dir := privateDir(t, 0o700)
			path := filepath.Join(dir, "o.json")
			if err := os.Symlink(filepath.Join(dir, "elsewhere"), path); err != nil {
				t.Fatal(err)
			}
			return path
		},
	}
	for name, setup := range rejections {
		t.Run("rejects "+name, func(t *testing.T) {
			path := setup(t)
			err := persistChildSmokeObservation(depsFor(path), result, 1)
			if err == nil || err.Error() != "child_smoke_observation_path_invalid" {
				t.Fatalf("err = %v", err)
			}
			if name == "existing file" {
				if data, _ := os.ReadFile(path); string(data) != "keep" {
					t.Fatalf("existing file was overwritten: %q", data)
				}
			}
		})
	}
	t.Run("symlinked parent", func(t *testing.T) {
		real := privateDir(t, 0o700)
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		err := persistChildSmokeObservation(depsFor(filepath.Join(link, "o.json")), result, 1)
		if err == nil || err.Error() != "child_smoke_observation_path_invalid" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestBoundedVersionKeepsFirstLineTrimmedAndCapped(t *testing.T) {
	if got := boundedVersion("  tool 1.2.3\nsecond line\n"); got != "tool 1.2.3" {
		t.Fatalf("got %q", got)
	}
	if got := boundedVersion(strings.Repeat("x", 1000)); len(got) > 256 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestPreflightClassifiesMissingAndFailingExecutables(t *testing.T) {
	request := port.HostProbeRequest{Model: "m"}
	notFound := normalizeDependencies(Dependencies{LookPath: func(string) (string, error) { return "", errors.New("nf") }})
	got := preflight(t.Context(), notFound, "codex", "codex", request, nil)
	if got.Installed || got.Ready || got.Cause != "harness_environment" || got.Code != "executable_not_found" || got.EvidenceSource != "codex_preflight" || got.RequestedModel != "m" {
		t.Fatalf("not found: %+v", got)
	}
	failing := normalizeDependencies(Dependencies{
		LookPath: func(string) (string, error) { return "/x/codex", nil },
		Process:  fakeRunner{err: errors.New("command_failed")},
	})
	got = preflight(t.Context(), failing, "codex", "codex", request, nil)
	if !got.Installed || got.Ready || got.Code != "version_probe_failed" || got.Cause != "harness_environment" {
		t.Fatalf("version failure: %+v", got)
	}
	ok := normalizeDependencies(Dependencies{
		LookPath: func(string) (string, error) { return "/x/codex", nil },
		Process:  fakeRunner{out: CommandOutput{Stdout: []byte("codex 9.9\nextra")}},
	})
	got = preflight(t.Context(), ok, "codex", "codex", request, nil)
	if !got.Installed || !got.Ready || got.Version != "codex 9.9" || got.Code != "" {
		t.Fatalf("ready: %+v", got)
	}
}

type fakeRunner struct {
	out CommandOutput
	err error
}

func (f fakeRunner) Run(context.Context, CommandRequest) (CommandOutput, error) { return f.out, f.err }

func TestIsolatedHostEnvKeepsOnlyAmbientAndAllowedSorted(t *testing.T) {
	deps := Dependencies{Environ: func() []string {
		return []string{"ZED=1", "PATH=/bin", "SECRET=x", "malformed", "LC_ALL=C", "HOME=/h", "EXTRA=e", "PATH2=no"}
	}}
	got := isolatedHostEnv(deps, "EXTRA")
	want := []string{"EXTRA=e", "HOME=/h", "LC_ALL=C", "PATH=/bin"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFailedAndCompletedResultFallBackToRequestVersion(t *testing.T) {
	request := captureRequest()
	request.HostVersion = "req-v"
	request.Model = "m"
	now := time.Unix(100, 0)
	deps := Dependencies{Now: func() time.Time { return now.Add(1500 * time.Millisecond) }}
	failed := failedResult("codex", "", request, now, deps, "transport", "x")
	if failed.Completed || failed.HostVersion != "req-v" || failed.DurationMS != 1500 || failed.EvidenceSource != "codex_runner" || failed.Cause != "transport" || failed.Code != "x" || failed.SchemaSHA256 != request.SchemaSHA256 {
		t.Fatalf("failed = %+v", failed)
	}
	if got := failedResult("codex", "live", request, now, deps, "c", "d"); got.HostVersion != "live" {
		t.Fatalf("version = %q", got.HostVersion)
	}
	capture := episodeCapture{FixtureID: "fx", CallCount: 2, RawSHA256: "r", CanonicalArguments: map[string]any{"a": 1}, SchemaSHA256: "s", RunTokenSHA256: "t", Classification: "multiple_calls", Diagnostics: nil}
	done := completedResult("codex", request, now, deps, capture)
	if !done.Completed || done.HostVersion != "req-v" || done.CallCount != 2 || done.CanonicalArgumentsJSON != `{"a":1}` || done.DiagnosticsJSON != "null" || done.EvidenceID != "t" || done.Classification != "multiple_calls" || done.DurationMS != 1500 {
		t.Fatalf("completed = %+v", done)
	}
}
