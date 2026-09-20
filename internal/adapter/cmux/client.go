package cmux

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const commandTimeout = 5 * time.Second

var (
	cmuxVersionPattern      = regexp.MustCompile(`^cmux ([0-9]+\.[0-9]+\.[0-9]+)(?:\s|$)`)
	cmuxWorkspaceRefPattern = regexp.MustCompile(`^workspace:[0-9]+$`)
)

type CommandRequest struct {
	Executable string
	Args       []string
	Env        map[string]string
	Timeout    time.Duration
}

type CommandOutput struct {
	Stdout  []byte
	Stderr  []byte
	Invoked bool
}

type Runner interface {
	Run(context.Context, CommandRequest) (CommandOutput, error)
}

type EndpointObserver func(path string, expectedUID int) (EndpointIncarnation, error)

type Client struct {
	Runner          Runner
	ObserveEndpoint EndpointObserver
	UID             int
}

type PreflightRequest struct {
	Executable      string
	ExpectedVersion string
	SocketPath      string
	WindowID        string
}

type PreflightResult struct {
	Executable string
	Version    string
	SocketPath string
	WindowID   string
	Endpoint   EndpointIncarnation
}

type CreateRequest struct {
	Preflight PreflightResult
	AttemptID string
	CWD       string
}

type CreatedWorkspace struct {
	Preflight   PreflightResult
	WindowID    string
	WorkspaceID string
	PaneID      string
	SurfaceID   string
	CWD         string
	CreateMS    uint64
	ResolveMS   uint64
}

type SendRequest struct {
	Created CreatedWorkspace
	Command string
}

type SendReceipt struct {
	Accepted bool
	SendMS   uint64
}

type MutationError struct {
	Phase     string
	Ambiguous bool
	Cause     error
}

func (err *MutationError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("cmux %s failed: %v", err.Phase, err.Cause)
}

func (err *MutationError) Unwrap() error { return err.Cause }

func (client Client) Preflight(ctx context.Context, request PreflightRequest) (PreflightResult, error) {
	if client.Runner == nil || client.ObserveEndpoint == nil {
		return PreflightResult{}, fmt.Errorf("cmux preflight dependencies are unavailable")
	}
	if !filepath.IsAbs(request.Executable) || filepath.Clean(request.Executable) != request.Executable {
		return PreflightResult{}, fmt.Errorf("cmux executable must be an absolute clean path")
	}
	if request.ExpectedVersion == "" || request.SocketPath == "" || !validUUID(request.WindowID) {
		return PreflightResult{}, fmt.Errorf("cmux preflight identity is incomplete")
	}
	before, err := client.ObserveEndpoint(request.SocketPath, client.UID)
	if err != nil {
		return PreflightResult{}, err
	}
	versionOutput, err := client.run(ctx, request.Executable, request.SocketPath, "version")
	if err != nil {
		return PreflightResult{}, fmt.Errorf("cmux version preflight failed: %w", err)
	}
	match := cmuxVersionPattern.FindStringSubmatch(strings.TrimSpace(string(versionOutput.Stdout)))
	if len(match) != 2 || match[1] != request.ExpectedVersion {
		return PreflightResult{}, fmt.Errorf("cmux version mismatch: expected %s", request.ExpectedVersion)
	}
	ping, err := client.run(ctx, request.Executable, request.SocketPath, "ping")
	if err != nil || strings.TrimSpace(string(ping.Stdout)) != "PONG" {
		return PreflightResult{}, fmt.Errorf("cmux ping preflight failed")
	}
	capabilityOutput, err := client.run(ctx, request.Executable, request.SocketPath, "capabilities")
	if err != nil {
		return PreflightResult{}, fmt.Errorf("cmux capabilities preflight failed: %w", err)
	}
	if err := validateCapabilities(capabilityOutput.Stdout, request.SocketPath); err != nil {
		return PreflightResult{}, err
	}
	identifyOutput, err := client.run(ctx, request.Executable, request.SocketPath,
		"--id-format", "uuids", "identify", "--window", request.WindowID, "--no-caller")
	if err != nil {
		return PreflightResult{}, fmt.Errorf("cmux identify preflight failed: %w", err)
	}
	if err := validateIdentify(identifyOutput.Stdout, request.SocketPath, request.WindowID, "", "", false); err != nil {
		return PreflightResult{}, err
	}
	after, err := client.ObserveEndpoint(request.SocketPath, client.UID)
	if err != nil {
		return PreflightResult{}, err
	}
	if !SameEndpoint(before, after) {
		return PreflightResult{}, fmt.Errorf("cmux socket endpoint incarnation changed during preflight")
	}
	return PreflightResult{Executable: request.Executable, Version: request.ExpectedVersion, SocketPath: request.SocketPath, WindowID: request.WindowID, Endpoint: before}, nil
}

func (client Client) CreateWorkspace(ctx context.Context, request CreateRequest) (CreatedWorkspace, error) {
	if !validUUID(request.Preflight.WindowID) || !filepath.IsAbs(request.CWD) || filepath.Clean(request.CWD) != request.CWD || strings.TrimSpace(request.AttemptID) == "" {
		return CreatedWorkspace{}, fmt.Errorf("cmux create request identity is invalid")
	}
	if err := client.ensureEndpoint(request.Preflight); err != nil {
		return CreatedWorkspace{}, err
	}
	name := workspaceName(request.AttemptID)
	createStarted := time.Now()
	output, err := client.run(ctx, request.Preflight.Executable, request.Preflight.SocketPath,
		"new-workspace", "--name", name, "--cwd", request.CWD, "--window", request.Preflight.WindowID, "--focus", "false")
	createMS := elapsedMS(createStarted)
	if err != nil {
		return CreatedWorkspace{}, &MutationError{Phase: "workspace_create", Ambiguous: output.Invoked, Cause: err}
	}
	createdHandle, err := parseCreatedWorkspaceHandle(output.Stdout)
	if err != nil {
		return CreatedWorkspace{}, &MutationError{Phase: "workspace_create", Ambiguous: true, Cause: fmt.Errorf("malformed create response")}
	}
	result := CreatedWorkspace{Preflight: request.Preflight, WindowID: request.Preflight.WindowID, CWD: request.CWD, CreateMS: createMS}
	resolveStarted := time.Now()
	if err := client.resolveCreatedTarget(ctx, name, createdHandle, &result); err != nil {
		return result, &MutationError{Phase: "target_resolve", Ambiguous: true, Cause: err}
	}
	result.ResolveMS = elapsedMS(resolveStarted)
	if err := client.ensureEndpoint(request.Preflight); err != nil {
		return result, &MutationError{Phase: "target_resolve", Ambiguous: true, Cause: err}
	}
	return result, nil
}

func (client Client) Send(ctx context.Context, request SendRequest) (SendReceipt, error) {
	created := request.Created
	if !validUUID(created.WindowID) || !validUUID(created.WorkspaceID) || !validUUID(created.SurfaceID) || strings.TrimSpace(request.Command) == "" {
		return SendReceipt{}, fmt.Errorf("cmux send target is incomplete")
	}
	if err := client.ensureEndpoint(created.Preflight); err != nil {
		return SendReceipt{}, err
	}
	started := time.Now()
	output, err := client.run(ctx, created.Preflight.Executable, created.Preflight.SocketPath,
		"--json", "--id-format", "uuids", "send", "--window", created.WindowID,
		"--workspace", created.WorkspaceID, "--surface", created.SurfaceID, "--", request.Command+`\n`)
	receipt := SendReceipt{SendMS: elapsedMS(started)}
	if err != nil {
		return receipt, &MutationError{Phase: "input_send", Ambiguous: output.Invoked, Cause: err}
	}
	var accepted struct {
		WindowID    string `json:"window_id"`
		WorkspaceID string `json:"workspace_id"`
		SurfaceID   string `json:"surface_id"`
		Queued      *bool  `json:"queued"`
	}
	if err := decodeStrict(output.Stdout, &accepted); err != nil {
		return receipt, &MutationError{Phase: "input_send", Ambiguous: true, Cause: fmt.Errorf("malformed send response: %w", err)}
	}
	if accepted.WindowID != created.WindowID || accepted.WorkspaceID != created.WorkspaceID ||
		accepted.SurfaceID != created.SurfaceID || accepted.Queued == nil {
		return receipt, &MutationError{Phase: "input_send", Ambiguous: true, Cause: fmt.Errorf("cmux send receipt target identity is incomplete or mismatched")}
	}
	if err := client.ensureEndpoint(created.Preflight); err != nil {
		return receipt, &MutationError{Phase: "input_send", Ambiguous: true, Cause: err}
	}
	receipt.Accepted = true
	return receipt, nil
}

func (client Client) resolveCreatedTarget(ctx context.Context, name, createdHandle string, result *CreatedWorkspace) error {
	workspaceOutput, err := client.run(ctx, result.Preflight.Executable, result.Preflight.SocketPath,
		"--json", "--id-format", "both", "list-workspaces", "--window", result.WindowID)
	if err != nil {
		return err
	}
	var workspaces struct {
		Workspaces []struct{ ID, Ref, Title string } `json:"workspaces"`
	}
	if err := decodeStrict(workspaceOutput.Stdout, &workspaces); err != nil {
		return fmt.Errorf("malformed workspace list: %w", err)
	}
	for _, workspace := range workspaces.Workspaces {
		if workspace.Title == name && (workspace.Ref == createdHandle || workspace.ID == createdHandle) {
			if result.WorkspaceID != "" {
				return fmt.Errorf("duplicate created workspace identity")
			}
			result.WorkspaceID = workspace.ID
		}
	}
	if !validUUID(result.WorkspaceID) {
		return fmt.Errorf("created workspace identity is unavailable")
	}
	paneOutput, err := client.run(ctx, result.Preflight.Executable, result.Preflight.SocketPath,
		"--json", "--id-format", "uuids", "list-panes", "--window", result.WindowID, "--workspace", result.WorkspaceID)
	if err != nil {
		return err
	}
	var panes struct {
		Panes []struct {
			ID           string `json:"id"`
			SurfaceCount int    `json:"surface_count"`
		} `json:"panes"`
	}
	if err := decodeStrict(paneOutput.Stdout, &panes); err != nil || len(panes.Panes) != 1 || panes.Panes[0].SurfaceCount != 1 || !validUUID(panes.Panes[0].ID) {
		return fmt.Errorf("created workspace pane identity is incomplete")
	}
	result.PaneID = panes.Panes[0].ID
	surfaceOutput, err := client.run(ctx, result.Preflight.Executable, result.Preflight.SocketPath,
		"--json", "--id-format", "uuids", "list-pane-surfaces", "--window", result.WindowID,
		"--workspace", result.WorkspaceID, "--pane", result.PaneID)
	if err != nil {
		return err
	}
	var surfaces struct {
		Surfaces []struct {
			ID string `json:"id"`
		} `json:"surfaces"`
	}
	if err := decodeStrict(surfaceOutput.Stdout, &surfaces); err != nil || len(surfaces.Surfaces) != 1 || !validUUID(surfaces.Surfaces[0].ID) {
		return fmt.Errorf("created workspace surface identity is incomplete")
	}
	result.SurfaceID = surfaces.Surfaces[0].ID
	identifyOutput, err := client.run(ctx, result.Preflight.Executable, result.Preflight.SocketPath,
		"--id-format", "uuids", "identify", "--window", result.WindowID,
		"--workspace", result.WorkspaceID, "--surface", result.SurfaceID)
	if err != nil {
		return err
	}
	return validateIdentify(identifyOutput.Stdout, result.Preflight.SocketPath, result.WindowID, result.WorkspaceID, result.SurfaceID, true)
}

func parseCreatedWorkspaceHandle(data []byte) (string, error) {
	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) != 2 || fields[0] != "OK" ||
		(!cmuxWorkspaceRefPattern.MatchString(fields[1]) && !validUUID(fields[1])) {
		return "", fmt.Errorf("cmux create response is malformed")
	}
	return fields[1], nil
}

func (client Client) ensureEndpoint(preflight PreflightResult) error {
	current, err := client.ObserveEndpoint(preflight.SocketPath, client.UID)
	if err != nil {
		return err
	}
	if !SameEndpoint(preflight.Endpoint, current) {
		return fmt.Errorf("cmux socket endpoint incarnation changed")
	}
	return nil
}

func (client Client) run(ctx context.Context, executable, socketPath string, args ...string) (CommandOutput, error) {
	return client.Runner.Run(ctx, CommandRequest{
		Executable: executable, Args: append([]string{"--socket", socketPath}, args...), Timeout: commandTimeout,
		Env: map[string]string{"CMUX_SOCKET_PATH": socketPath, "CMUX_SOCKET": "", "CMUX_WORKSPACE_ID": "", "CMUX_SURFACE_ID": ""},
	})
}

func workspaceName(attemptID string) string {
	sum := sha256.Sum256([]byte(attemptID))
	return "issueops-" + hex.EncodeToString(sum[:6])
}

func elapsedMS(start time.Time) uint64 {
	value := uint64(time.Since(start).Milliseconds())
	if value == 0 {
		return 1
	}
	return value
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}

type capabilitiesResponse struct {
	Protocol   string   `json:"protocol"`
	Version    int      `json:"version"`
	SocketPath string   `json:"socket_path"`
	AccessMode string   `json:"access_mode"`
	Methods    []string `json:"methods"`
}

func validateCapabilities(data []byte, socketPath string) error {
	var response capabilitiesResponse
	if err := decodeStrict(data, &response); err != nil {
		return fmt.Errorf("cmux capabilities response is malformed: %w", err)
	}
	if response.Protocol != "cmux-socket" || response.Version != 2 || response.SocketPath != socketPath || response.AccessMode == "" {
		return fmt.Errorf("cmux capabilities identity is incomplete")
	}
	seen := map[string]bool{}
	for _, method := range response.Methods {
		if seen[method] {
			return fmt.Errorf("cmux capabilities contain duplicate method %q", method)
		}
		seen[method] = true
	}
	for _, required := range []string{"system.ping", "system.capabilities", "system.identify", "workspace.create", "workspace.list", "pane.list", "pane.surfaces", "surface.send_text"} {
		if !seen[required] {
			return fmt.Errorf("cmux required capability %s is unavailable", required)
		}
	}
	return nil
}

type identifyResponse struct {
	SocketPath string `json:"socket_path"`
	Focused    *struct {
		WindowID string `json:"window_id"`
	} `json:"focused"`
	Caller *struct {
		WorkspaceID string `json:"workspace_id"`
		SurfaceID   string `json:"surface_id"`
	} `json:"caller"`
}

func validateIdentify(data []byte, socketPath, windowID, workspaceID, surfaceID string, requireCaller bool) error {
	var response identifyResponse
	if err := decodeStrict(data, &response); err != nil {
		return fmt.Errorf("cmux identify response is malformed: %w", err)
	}
	if response.SocketPath != socketPath || response.Focused == nil || response.Focused.WindowID != windowID {
		return fmt.Errorf("cmux identify window identity mismatch")
	}
	if !requireCaller {
		if response.Caller != nil {
			return fmt.Errorf("cmux identify unexpectedly returned caller identity")
		}
		return nil
	}
	if response.Caller == nil || response.Caller.WorkspaceID != workspaceID || response.Caller.SurfaceID != surfaceID {
		return fmt.Errorf("cmux identify target identity mismatch")
	}
	return nil
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	value, err := decodeJSONValue(decoder)
	if err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, destination)
}

func decodeJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("JSON object key is not a string")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate JSON key %q", key)
			}
			object[key], err = decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("malformed JSON object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			item, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, item)
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("malformed JSON array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter")
	}
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, request CommandRequest) (CommandOutput, error) {
	runCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, request.Executable, request.Args...)
	command.Env = commandEnvironment(request.Env)
	var stdout, stderr boundedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Start(); err != nil {
		return CommandOutput{}, err
	}
	output := CommandOutput{Invoked: true}
	err := command.Wait()
	output.Stdout, output.Stderr = stdout.Bytes(), stderr.Bytes()
	if stdout.exceeded || stderr.exceeded {
		return output, fmt.Errorf("cmux command output exceeded 1 MiB")
	}
	if err != nil {
		return output, fmt.Errorf("cmux command failed: %w: %s", err, strings.TrimSpace(string(output.Stderr)))
	}
	return output, nil
}

type boundedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	const limit = 1 << 20
	original := len(value)
	if buffer.Len()+len(value) > limit {
		value = value[:max(0, limit-buffer.Len())]
		buffer.exceeded = true
	}
	_, err := buffer.Buffer.Write(value)
	return original, err
}

func commandEnvironment(overrides map[string]string) []string {
	blocked := map[string]bool{"CMUX_SOCKET_PATH": true, "CMUX_SOCKET": true, "CMUX_WORKSPACE_ID": true, "CMUX_SURFACE_ID": true}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !blocked[name] {
			environment = append(environment, entry)
		}
	}
	for _, name := range []string{"CMUX_SOCKET_PATH", "CMUX_SOCKET", "CMUX_WORKSPACE_ID", "CMUX_SURFACE_ID"} {
		if value := overrides[name]; value != "" {
			environment = append(environment, name+"="+value)
		}
	}
	return environment
}
