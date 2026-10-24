package verifywork

import (
	"os/exec"

	"issueops/internal/adapter/outbound/gitbin"
)

func GitStatus(root string) (string, error) {
	output, err := exec.Command(gitbin.Path(), "-C", root, "status", "--short").Output()
	return string(output), err
}
