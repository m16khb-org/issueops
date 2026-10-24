package commitsuggest

import (
	"os/exec"

	"issueops/internal/adapter/outbound/gitbin"
)

type Effects struct{ Normalize func(string) (string, error) }

func (effects Effects) NormalizeRoot(root string) (string, error) { return effects.Normalize(root) }

func (Effects) Diff(root string, staged bool) (string, error) {
	args := []string{"-C", root, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	output, err := exec.Command(gitbin.Path(), args...).Output()
	return string(output), err
}
