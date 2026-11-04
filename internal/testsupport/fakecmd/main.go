// Command fakecmd lets a test's POSIX shell script stand in for a native
// command on Windows. testsupport.WriteFakeCommand copies this binary to
// NAME.exe next to NAME.sh and NAME.run.sh and writes the bash path to
// NAME.exe.bash. Running NAME.exe runs NAME.sh with the same argv, stdio, and
// exit status.
//
// The MSYS runtime re-parses a native command line: it glob- and
// brace-expands words and, with globbing off, stops unescaping `\"`. So argv
// never goes on bash's command line. The shim writes it NUL-separated to a
// temporary file and NAME.run.sh reads it back with mapfile.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() { os.Exit(shim()) }

// shim runs NAME.sh and returns the exit status the process ends with.
func shim() int {
	self, err := os.Executable()
	if err != nil {
		return fail(err)
	}
	bash, err := os.ReadFile(self + ".bash")
	if err != nil {
		return fail(err)
	}
	// Next to the shim, not in TMP: runners may strip TMP/TEMP from the env.
	args, err := os.CreateTemp(filepath.Dir(self), ".fakecmd-args-*")
	if err != nil {
		return fail(err)
	}
	for _, arg := range os.Args[1:] {
		if _, err := args.WriteString(arg + "\x00"); err != nil {
			return fail(err)
		}
	}
	if err := args.Close(); err != nil {
		return fail(err)
	}
	status := run(strings.TrimSpace(string(bash)), strings.TrimSuffix(self, ".exe")+".run.sh", args.Name())
	_ = os.Remove(args.Name())
	return status
}

func run(bash, runner, args string) int {
	command := exec.Command(bash, runner)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Keep MSYS from rewriting arguments that look like POSIX paths when the
	// script execs a native program.
	command.Env = append(os.Environ(), "FAKECMD_ARGS="+args, "MSYS_NO_PATHCONV=1", "MSYS2_ARG_CONV_EXCL=*")
	if err := command.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode()
		}
		return fail(err)
	}
	return 0
}

// fail reports err and returns the status of a shim that could not run.
func fail(err error) int {
	fmt.Fprintln(os.Stderr, "fakecmd:", err)
	return 127
}
