package hostprobe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"issueops/internal/port"

	_ "modernc.org/sqlite"
)

// ompToolNameLimit is omp's MCP tool name cap; longer names receive a Bun.hash suffix.
const ompToolNameLimit = 64

var (
	ompToolNameInvalid    = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`[^a-z0-9_]+`) })
	ompToolNameUnderscore = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`_+`) })
)

// OmpRunner runs one capture-only MCP episode in an isolated oh-my-pi session.
type OmpRunner struct {
	canonicalExtension func(string) string
	harnessBinary      string
	lifecycleExtension string
	deps               Dependencies
}

type ompMCPConfig struct {
	MCPServers map[string]ompMCPServer `json:"mcpServers"`
}

type ompMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
}

func NewOmpRunner(harnessBinary, lifecycleExtension string, deps Dependencies, canonicalExtension func(string) string) OmpRunner {
	return OmpRunner{canonicalExtension: canonicalExtension,
		harnessBinary:      harnessBinary,
		lifecycleExtension: lifecycleExtension,
		deps:               normalizeDependencies(deps),
	}
}

func (OmpRunner) Name() string { return "omp" }

func (r OmpRunner) Preflight(ctx context.Context, request port.HostProbeRequest) (result port.HostProbePreflight) {
	result = port.HostProbePreflight{
		Host:           r.Name(),
		RequestedModel: request.Model,
		EvidenceSource: "omp_preflight",
	}
	executable, err := r.resolveExecutable()
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "executable_not_found"
		return result
	}
	result.Installed = true
	authSource, err := resolveOmpAuth(r.deps)
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "auth_copy_failed"
		return result
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = episodeRootFailureCode(err)
		return result
	}
	defer func() {
		if err := r.deps.RemoveAll(root); err != nil {
			result.Ready = false
			result.MockExtensionVerified = false
			result.Cause = "harness_environment"
			result.Code = "private_root_cleanup_failed"
			result.EvidenceSource = "omp_preflight"
		}
	}()
	if err := prepareOmpPrivateState(root); err != nil {
		result.Cause = "harness_environment"
		result.Code = "episode_prepare_failed"
		return result
	}
	if err := snapshotOmpAuth(ctx, authSource, filepath.Join(root, "agent", "agent.db")); err != nil {
		result.Cause = "harness_environment"
		result.Code = "auth_copy_failed"
		return result
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Argv:    []string{executable, "--version"},
		Env:     isolatedOmpEnv(r.deps, root),
		Timeout: VersionTimeout,
	})
	if err != nil || output.ExitCode != 0 {
		result.Cause = "harness_environment"
		result.Code = "version_probe_failed"
		return result
	}
	result.Ready = true
	result.Version = boundedVersion(string(output.Stdout))
	result.MockExtensionVerified = r.validLifecycleExtension()
	if result.Ready && !result.MockExtensionVerified {
		result.Ready = false
		result.Cause = "harness_environment"
		result.Code = "mock_extension_invalid"
		result.EvidenceSource = "omp_preflight"
	}
	if result.Ready && !explicitOmoModel(request.Model) {
		result.Ready = false
		result.Cause = "harness_environment"
		result.Code = "explicit_model_required"
		result.EvidenceSource = "omp_preflight"
	}
	return result
}

func (r OmpRunner) resolveExecutable() (string, error) {
	executable, err := r.deps.LookPath("omp")
	if err != nil {
		return "", err
	}
	executable, err = filepath.Abs(executable)
	if err != nil || !filepath.IsAbs(executable) {
		return "", fmt.Errorf("omp_executable_invalid")
	}
	return executable, nil
}

func (r OmpRunner) Run(ctx context.Context, request port.HostProbeRequest) (result port.HostProbeResult) {
	started := r.deps.Now()
	if r.harnessBinary == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_missing")
	}
	if strings.TrimSpace(r.lifecycleExtension) == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "lifecycle_extension_missing")
	}
	if !r.validLifecycleExtension() {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "mock_extension_invalid")
	}
	if !explicitOmoModel(request.Model) {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "explicit_model_required")
	}
	targetTool, ok := ompProbeToolName(request.ProbeTool)
	if !ok {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "probe_tool_name_too_long")
	}
	harnessBinary, err := filepath.Abs(r.harnessBinary)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_path_invalid")
	}
	request.HarnessBinary = harnessBinary
	executable, err := r.resolveExecutable()
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "executable_not_found")
	}
	authSource, err := resolveOmpAuth(r.deps)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "auth_copy_failed")
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", episodeRootFailureCode(err))
	}
	defer func() {
		if err := r.deps.RemoveAll(root); err != nil {
			result = failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "private_root_cleanup_failed")
		}
	}()

	if err := prepareOmpPrivateState(root); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "episode_prepare_failed")
	}
	if err := snapshotOmpAuth(ctx, authSource, filepath.Join(root, "agent", "agent.db")); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "auth_copy_failed")
	}
	resultPath := filepath.Join(root, "result.json")
	if err := prepareOmpEpisode(root, r.lifecycleExtension, targetTool, request, resultPath); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "episode_prepare_failed")
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     root,
		Argv:    ompArgv(executable, root, request),
		Env:     isolatedOmpEnv(r.deps, root),
		Timeout: EpisodeTimeout,
		// omp emits the same pi agent event stream as Omo.
		OmoJSONL: true,
	})
	if err != nil {
		cause, code := normalizedProcessFailure(err, "host_process_failed")
		result := failedResult(r.Name(), "", request, started, r.deps, cause, code)
		result.ExitCode = output.ExitCode
		return result
	}
	if output.ExitCode != 0 {
		result := failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "host_process_failed")
		result.ExitCode = output.ExitCode
		return result
	}
	observation, err := observeOmoStream(output.Stdout, targetTool)
	if err != nil {
		cause, code := omoStreamFailure(err)
		result := failedResult(r.Name(), "", request, started, r.deps, cause, code)
		result.ExitCode = output.ExitCode
		result.AmbientToolCount = observation.AmbientToolCount
		result.CallCount = observation.MCPCallCount
		return result
	}
	capture, err := decodeEpisodeCapture(resultPath, request)
	if err != nil {
		cause, code := normalizedCaptureFailure(err)
		result := failedResult(r.Name(), "", request, started, r.deps, cause, code)
		result.ExitCode = output.ExitCode
		return result
	}
	if capture.CallCount != 1 || observation.MCPCallCount != 1 {
		result := failedResult(r.Name(), "", request, started, r.deps, "transport", "probe_call_cardinality_invalid")
		result.ExitCode = output.ExitCode
		return result
	}
	result = completedResult(r.Name(), "", request, started, r.deps, capture)
	result.ObservedModel = observation.Model
	// The private guard extension blocks this exact MCP tool unless the managed
	// lifecycle extension injected one hidden project-doc message first.
	result.SessionStartObserved = true
	result.ResponseSHA256 = observation.ResponseSHA256
	result.AmbientToolCount = observation.AmbientToolCount
	result.ExitCode = output.ExitCode
	return result
}

func (r OmpRunner) validLifecycleExtension() bool {
	absolute, err := filepath.Abs(r.harnessBinary)
	return err == nil && r.canonicalExtension != nil && r.lifecycleExtension == r.canonicalExtension(absolute)
}

func prepareOmpEpisode(root, lifecycleExtension, targetTool string, request port.HostProbeRequest, resultPath string) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(request.HarnessBinary) || !filepath.IsAbs(resultPath) {
		return fmt.Errorf("episode_path_invalid")
	}
	if err := writePrivateFile(filepath.Join(root, "issueops-lifecycle.js"), []byte(lifecycleExtension)); err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Join(root, "issueops-conformance.js"), []byte(omoContextGuardExtension(targetTool))); err != nil {
		return err
	}
	const projectDoc = "# Omp conformance context\n\nThis private document proves lifecycle context injection for one isolated host probe.\n"
	if err := writePrivateFile(filepath.Join(root, ".issueops", "CONSTITUTION.md"), []byte(projectDoc)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "sessions"), 0o700); err != nil {
		return err
	}
	serve := serveArgv(request, resultPath)
	data, err := json.Marshal(ompMCPConfig{MCPServers: map[string]ompMCPServer{
		omoProbeServer: {Type: "stdio", Command: serve[0], Args: serve[1:], Cwd: root},
	}})
	if err != nil {
		return err
	}
	return writePrivateFile(filepath.Join(root, "agent", "mcp.json"), data)
}

// resolveOmpAuth returns the default-profile omp credential store, or "" when none exists.
func resolveOmpAuth(deps Dependencies) (string, error) {
	sourceAgentDir := strings.TrimSpace(deps.Getenv("PI_CODING_AGENT_DIR"))
	if sourceAgentDir == "" {
		home := strings.TrimSpace(deps.Getenv("HOME"))
		if home == "" {
			return "", nil
		}
		if !filepath.IsAbs(home) {
			return "", fmt.Errorf("omp_home_invalid")
		}
		sourceAgentDir = filepath.Join(home, ".omp", "agent")
	}
	if !filepath.IsAbs(sourceAgentDir) {
		return "", fmt.Errorf("omp_agent_dir_invalid")
	}
	source := filepath.Join(sourceAgentDir, "agent.db")
	info, err := os.Lstat(source)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return "", fmt.Errorf("omp_auth_invalid")
	}
	return source, nil
}

// snapshotOmpAuth copies only enabled model-provider credentials into a fresh
// private agent.db; omp creates its remaining tables on first open. MCP OAuth
// rows stay behind, and OAuth refresh tokens are blanked so the episode can use
// a live access token but never rotate the real store's refresh token.
func snapshotOmpAuth(ctx context.Context, source, destination string) error {
	if source == "" {
		return nil
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+destination+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE ? AS source", "file:"+source+"?mode=ro"); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, "SELECT sql FROM source.sqlite_master WHERE type = 'table' AND name IN ('auth_schema_version', 'auth_credentials')")
	if err != nil {
		return err
	}
	schema := []string{}
	for rows.Next() {
		var statement string
		if err := rows.Scan(&statement); err != nil {
			rows.Close()
			return err
		}
		schema = append(schema, statement)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if len(schema) == 0 {
		return nil
	}
	if len(schema) != 2 {
		return fmt.Errorf("omp_auth_invalid")
	}
	for _, statement := range append(schema,
		"INSERT INTO main.auth_schema_version SELECT * FROM source.auth_schema_version",
		"INSERT INTO main.auth_credentials SELECT * FROM source.auth_credentials WHERE disabled_cause IS NULL AND provider NOT LIKE 'mcp_oauth:%'",
		"UPDATE main.auth_credentials SET data = json_set(data, '$.refresh', '') WHERE credential_type = 'oauth'",
	) {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func prepareOmpPrivateState(root string) error {
	for _, directory := range []string{filepath.Join(root, "home"), filepath.Join(root, "agent")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// isolatedOmpEnv points every omp root at the private episode: HOME hides the
// real ~/.omp plus foreign Claude/Codex/Gemini configs and ~/.env, and
// PI_CODING_AGENT_DIR selects the private agent dir. OMP_MCP_REQUIRE_READY makes
// an unavailable probe server fail the run instead of reaching the model.
func isolatedOmpEnv(deps Dependencies, root string) []string {
	environment := replaceEnvironmentValue(isolatedHostEnv(deps), "HOME", filepath.Join(root, "home"))
	environment = replaceEnvironmentValue(environment, "PI_CODING_AGENT_DIR", filepath.Join(root, "agent"))
	return replaceEnvironmentValue(environment, "OMP_MCP_REQUIRE_READY", "1")
}

func ompArgv(executable, root string, request port.HostProbeRequest) []string {
	return []string{
		executable,
		"--mode", "json", "--print",
		"--model", request.Model,
		"--system-prompt", omoProbeSystemPrompt,
		"--no-session", "--session-dir", filepath.Join(root, "sessions"),
		"--no-tools",
		"--extension", filepath.Join(root, "issueops-lifecycle.js"),
		"--extension", filepath.Join(root, "issueops-conformance.js"),
		"--no-extensions", "--no-skills", "--no-rules", "--no-lsp", "--no-title", "--auto-approve",
		"--", request.Prompt,
	}
}

// ompProbeToolName mirrors omp's MCP tool naming: mcp__<server>_<tool> with both
// parts sanitized and a redundant "<server>_" tool prefix removed. Names over the
// limit get a Bun.hash suffix that is not reproduced here, so they are rejected.
func ompProbeToolName(tool string) (string, bool) {
	server := sanitizeOmpToolName(omoProbeServer, "server")
	name := "mcp__" + server + "_" + strings.TrimPrefix(sanitizeOmpToolName(tool, "tool"), server+"_")
	return name, len(name) <= ompToolNameLimit
}

func sanitizeOmpToolName(value, fallback string) string {
	value = ompToolNameInvalid().ReplaceAllString(strings.ToLower(value), "_")
	value = strings.Trim(ompToolNameUnderscore().ReplaceAllString(value, "_"), "_")
	if value == "" {
		return fallback
	}
	return value
}

var _ port.HostProbeRunner = OmpRunner{}
