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

var errLiveProbeObservationInvalid = fmt.Errorf("live_probe_observation_invalid")

func recordLiveProbeSessionStart(input []byte) error {
	if os.Getenv("ISSUEOPS_CHILD_SMOKE_HOOKS") != "1" {
		return nil
	}
	observationPath := strings.TrimSpace(os.Getenv("ISSUEOPS_CHILD_SMOKE_OBSERVATION_FILE"))
	if !filepath.IsAbs(observationPath) || len(input) == 0 || len(input) > maxLiveProbeHookInput {
		return errLiveProbeObservationInvalid
	}
	if !liveProbeObservationDirOK(filepath.Dir(observationPath)) {
		return errLiveProbeObservationInvalid
	}
	model, ok := decodeLiveProbeSessionStartModel(input)
	if !ok {
		return errLiveProbeObservationInvalid
	}
	data, err := json.Marshal(struct {
		Event string `json:"event"`
		Model string `json:"model,omitempty"`
	}{Event: "SessionStart", Model: model})
	if err != nil {
		return errLiveProbeObservationInvalid
	}
	data = append(data, '\n')
	return writeLiveProbeMarker(observationPath+".hooks", data)
}

// liveProbeObservationDirOK는 관찰 파일의 부모가 심볼릭 링크가 아닌 0700 디렉터리인지 확인한다.
func liveProbeObservationDirOK(dir string) bool {
	info, err := os.Lstat(dir)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0o700
}

// decodeLiveProbeSessionStartModel은 SessionStart 페이로드를 검증하고 model을 돌려준다.
func decodeLiveProbeSessionStartModel(input []byte) (string, bool) {
	var payload struct {
		HookEventName string `json:"hook_event_name"`
		Model         string `json:"model"`
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	if err := decoder.Decode(&payload); err != nil || payload.HookEventName != "SessionStart" ||
		strings.TrimSpace(payload.Model) != payload.Model || len(payload.Model) > 256 || strings.ContainsAny(payload.Model, "\r\n") {
		return "", false
	}
	return payload.Model, true
}

// writeLiveProbeMarker는 마커 파일을 배타적으로 만들고 실패 시 제거한다.
func writeLiveProbeMarker(markerPath string, data []byte) error {
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errLiveProbeObservationInvalid
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(markerPath)
		return errLiveProbeObservationInvalid
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(markerPath)
		return errLiveProbeObservationInvalid
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(markerPath)
		return errLiveProbeObservationInvalid
	}
	return nil
}
