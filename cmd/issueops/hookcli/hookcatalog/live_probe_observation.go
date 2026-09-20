package hookcatalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxLiveProbeHookInput = 64 << 10

func recordLiveProbeSessionStart(input []byte) error {
	if os.Getenv("ISSUEOPS_CHILD_SMOKE_HOOKS") != "1" {
		return nil
	}
	observationPath := strings.TrimSpace(os.Getenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"))
	if !filepath.IsAbs(observationPath) || len(input) == 0 || len(input) > maxLiveProbeHookInput {
		return fmt.Errorf("live_probe_observation_invalid")
	}
	parentInfo, err := os.Lstat(filepath.Dir(observationPath))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || parentInfo.Mode().Perm() != 0o700 {
		return fmt.Errorf("live_probe_observation_invalid")
	}
	var payload struct {
		HookEventName string `json:"hook_event_name"`
		Model         string `json:"model"`
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	if err := decoder.Decode(&payload); err != nil || payload.HookEventName != "SessionStart" || strings.TrimSpace(payload.Model) == "" ||
		strings.TrimSpace(payload.Model) != payload.Model || len(payload.Model) > 256 || strings.ContainsAny(payload.Model, "\r\n") {
		return fmt.Errorf("live_probe_observation_invalid")
	}
	data, err := json.Marshal(struct {
		Event string `json:"event"`
		Model string `json:"model"`
	}{Event: "SessionStart", Model: payload.Model})
	if err != nil {
		return fmt.Errorf("live_probe_observation_invalid")
	}
	data = append(data, '\n')
	markerPath := observationPath + ".hooks"
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("live_probe_observation_invalid")
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(markerPath)
		return fmt.Errorf("live_probe_observation_invalid")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(markerPath)
		return fmt.Errorf("live_probe_observation_invalid")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(markerPath)
		return fmt.Errorf("live_probe_observation_invalid")
	}
	return nil
}
