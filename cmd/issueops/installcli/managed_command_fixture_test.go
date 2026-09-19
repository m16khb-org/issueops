package installcli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type managedCommandFixture struct {
	source string
}

var managedTestCommandSource managedCommandFixture

func newManagedTestCommandFixture(build func(string) error) (managedCommandFixture, func() error, error) {
	directory, err := os.MkdirTemp("", "issueops-installcli-command-")
	if err != nil {
		return managedCommandFixture{}, nil, fmt.Errorf("create managed command fixture directory: %w", err)
	}
	source := filepath.Join(directory, "issueops")
	if err := build(source); err != nil {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			return managedCommandFixture{}, nil, fmt.Errorf("build managed command fixture: %w; cleanup: %v", err, cleanupErr)
		}
		return managedCommandFixture{}, nil, fmt.Errorf("build managed command fixture: %w", err)
	}
	info, err := os.Stat(source)
	if err != nil {
		_ = os.RemoveAll(directory)
		return managedCommandFixture{}, nil, fmt.Errorf("stat managed command fixture: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		_ = os.RemoveAll(directory)
		return managedCommandFixture{}, nil, fmt.Errorf("validate managed command fixture mode: %v", info.Mode())
	}
	return managedCommandFixture{source: source}, func() error { return os.RemoveAll(directory) }, nil
}

func runWithManagedCommandFixture(
	build func(string) error,
	run func(managedCommandFixture) int,
	report func(error),
) int {
	fixture, cleanup, err := newManagedTestCommandFixture(build)
	if err != nil {
		report(err)
		return 1
	}
	exitCode := run(fixture)
	if err := cleanup(); err != nil {
		report(fmt.Errorf("clean managed command fixture: %w", err))
		if exitCode == 0 {
			exitCode = 1
		}
	}
	return exitCode
}

func buildManagedTestCommandSource(target string) error {
	command := exec.Command("go", "build", "-o", target, "../../../cmd/issueops")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("go build managed test command: %w\n%s", err, output)
	}
	return nil
}

func (fixture managedCommandFixture) copyTo(destination string) (resultErr error) {
	source, err := os.Open(fixture.source)
	if err != nil {
		return fmt.Errorf("open managed command fixture: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat managed command fixture: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("managed command fixture is not a regular file: %s", fixture.source)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create managed command destination directory: %w", err)
	}
	target, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create managed command copy: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		return fmt.Errorf("copy managed command fixture: %w", err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close managed command copy: %w", err)
	}
	if err := os.Chmod(destination, info.Mode().Perm()); err != nil {
		return fmt.Errorf("set managed command copy mode: %w", err)
	}
	complete = true
	return nil
}
