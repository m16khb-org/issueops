package verifywork

import "os/exec"

func GitStatus(root string) (string, error) {
	output, err := exec.Command("git", "-C", root, "status", "--short").Output()
	return string(output), err
}
