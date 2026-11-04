package installutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/port"
)

// VerifyHookActivation performs semantic readback of only issueops hook
// groups while permitting unrelated co-resident hooks to remain untouched.
func VerifyHookActivation(path string, expected map[string]any) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var actual map[string]any
	if err := json.Unmarshal(raw, &actual); err != nil {
		return "", err
	}
	return VerifyHookConfigActivation(actual, expected)
}

// VerifyHookConfigActivation checks the parsed installed hook config actual
// against the expected catalog and returns the semantic SHA-256 of expected.
// Every expected event must be present and carry exactly one canonical
// issueops group; issueops groups on unexpected events are rejected, while
// unrelated third-party hook groups are ignored.
func VerifyHookConfigActivation(actual, expected map[string]any) (string, error) {
	expectedHooks, ok := expected["hooks"].(map[string]any)
	if !ok || len(expectedHooks) == 0 {
		return "", fmt.Errorf("expected hook catalog is empty")
	}
	actualHooks, ok := actual["hooks"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("installed hook catalog is missing")
	}
	expectedTargets := hookTargets(expected)
	if len(expectedTargets) == 1 {
		if messages := HookTargetDriftMessages(actual, "host", expectedTargets[0]); len(messages) > 0 {
			return "", fmt.Errorf("%s", messages[0])
		}
	}
	for event, groupsValue := range actualHooks {
		if err := verifyInstalledHookEvent(event, groupsValue, expectedHooks); err != nil {
			return "", err
		}
	}
	for event := range expectedHooks {
		if _, ok := actualHooks[event]; !ok {
			return "", fmt.Errorf("installed hook event %s is missing", event)
		}
	}
	return SemanticSHA256(expected)
}

func verifyInstalledHookEvent(event string, groupsValue any, expectedHooks map[string]any) error {
	groups, ok := groupsValue.([]any)
	if !ok {
		return fmt.Errorf("installed hook event %s is malformed", event)
	}
	harnessGroups := []any{}
	for _, group := range groups {
		if HookGroupContainsAgentHarness(group) {
			harnessGroups = append(harnessGroups, group)
		}
	}
	expectedGroupsValue, expectedEvent := expectedHooks[event]
	if !expectedEvent {
		if len(harnessGroups) != 0 {
			return fmt.Errorf("installed hook event %s contains an unexpected issueops group", event)
		}
		return nil
	}
	expectedGroups, ok := expectedGroupsValue.([]any)
	if !ok || len(expectedGroups) != 1 || len(harnessGroups) != 1 || !canonicalJSONEqual(harnessGroups[0], expectedGroups[0]) {
		return fmt.Errorf("installed hook event %s does not contain exactly one canonical issueops group", event)
	}
	return nil
}

// SemanticSHA256 returns the hex SHA-256 of the JSON encoding of value. Map
// keys marshal in sorted order, so equal structures hash equally.
func SemanticSHA256(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// CaptureNativeActivationEvidence reads the regular file at path once and
// returns its SHA-256, mode, size and device/inode identity together with the
// given host, surface and semanticSHA256. It fails if the file's identity
// changes between the lstat and the open read.
func CaptureNativeActivationEvidence(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	path = filepath.Clean(path)
	info, stat, err := lstatPhysicalRegularFile(path)
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return port.NativeActivationEvidence{}, err
	}
	opened, err := file.Stat()
	if err != nil {
		return port.NativeActivationEvidence{}, err
	}
	openedStat, ok := osfile.FileIdentity(file, opened)
	if !ok || !sameActivationReadback(opened, openedStat, info, stat) {
		return port.NativeActivationEvidence{}, fmt.Errorf("activation evidence changed during readback: %s", path)
	}
	return port.NativeActivationEvidence{
		Host: host, Surface: surface, Path: path, SemanticSHA256: semanticSHA256,
		SHA256: hex.EncodeToString(hash.Sum(nil)), Mode: uint32(opened.Mode()), Size: opened.Size(),
		Device: openedStat.Device, Inode: openedStat.Inode,
	}, nil
}

// lstatPhysicalRegularFile lstat이 돌려준 경로가 물리적 단일 링크 일반 파일인지 확인한다.
func lstatPhysicalRegularFile(path string) (os.FileInfo, osfile.Identity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, osfile.Identity{}, err
	}
	stat, ok := osfile.PathIdentity(path, info)
	if !ok || stat.Device == 0 || stat.Inode == 0 || stat.Links != 1 || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, osfile.Identity{}, fmt.Errorf("activation evidence must be a physical single-link regular file: %s", path)
	}
	return info, stat, nil
}

func sameActivationReadback(opened os.FileInfo, openedStat osfile.Identity, info os.FileInfo, stat osfile.Identity) bool {
	return openedStat.Device == stat.Device && openedStat.Inode == stat.Inode && opened.Size() == info.Size()
}

func canonicalJSONEqual(left, right any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	var leftValue, rightValue any
	if json.Unmarshal(leftRaw, &leftValue) != nil || json.Unmarshal(rightRaw, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}
