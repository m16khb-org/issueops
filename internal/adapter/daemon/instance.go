package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	contract "issueops/internal/contract/daemon"
	domain "issueops/internal/domain/daemon"
)

// InstanceRecord는 daemon 생명주기 동작을 정확히 하나의 OS 프로세스와 정확히
// 하나의 daemon protocol 인스턴스에 묶는다.

func ReadInstance(path string) (record contract.InstanceRecord, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return contract.InstanceRecord{}, err
	}
	if err := json.Unmarshal(b, &record); err != nil {
		return contract.InstanceRecord{}, fmt.Errorf("decode daemon instance record: %w", err)
	}
	if err := domain.ValidateInstance(record); err != nil {
		return contract.InstanceRecord{}, fmt.Errorf("invalid daemon instance record: %w", err)
	}
	return record, nil
}

func WriteInstance(path string, record contract.InstanceRecord) error {
	if err := domain.ValidateInstance(record); err != nil {
		return err
	}
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

func canonicalExecutable(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("process executable is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
