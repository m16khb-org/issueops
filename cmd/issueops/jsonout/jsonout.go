// Package jsonout writes the indented JSON every issueops CLI command prints.
// It is a leaf: it imports no other cmd package, so any command package and
// the issueopsapp composition root can share it without an import cycle.
package jsonout

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

// Print writes value to the current os.Stdout as two-space indented JSON.
func Print(value any) error {
	return PrintTo(os.Stdout, value)
}

// PrintTo writes value to w as two-space indented JSON.
func PrintTo(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// reportedError marks an error whose {"ok":false,"error":...} envelope is
// already on stdout. The root command must not repeat it on stderr: callers
// run `issueops ... --json 2>&1` and need exactly one JSON document.
type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

// Reported marks err as already printed on stdout. A nil err stays nil.
func Reported(err error) error {
	if err == nil {
		return nil
	}
	return reportedError{err: err}
}

// IsReported reports whether err (or an error it wraps) was already printed.
func IsReported(err error) bool {
	_, ok := errors.AsType[reportedError](err)
	return ok
}

// PrintError prints the {"ok":false,"error":...} envelope for err on stdout
// and returns err marked as reported. If printing fails, that error wins.
func PrintError(err error) error {
	if err == nil {
		return nil
	}
	if printErr := Print(map[string]any{"ok": false, "error": err.Error()}); printErr != nil {
		return printErr
	}
	return Reported(err)
}
