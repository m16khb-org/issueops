package lintdiagnose

import "os/exec"

type Effects struct{ Normalize func(string) (string, error) }

func (effects Effects) NormalizeRoot(root string) (string, error) { return effects.Normalize(root) }
func (Effects) Run(root string, argv []string) (string, int, bool) {
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0, false
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return string(output), exitError.ExitCode(), true
	}
	return string(output), -1, true
}
