// Package jsonout writes the indented JSON every issueops CLI command prints.
// It is a leaf: it imports no other cmd package, so any command package and
// the issueopsapp composition root can share it without an import cycle.
package jsonout

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync/atomic"
)

// stdoutDocument records that a JSON document reached os.Stdout in this
// command run, so the root never prints a second document after it.
var stdoutDocument atomic.Bool

// Print writes value to the current os.Stdout as two-space indented JSON.
func Print(value any) error {
	return PrintTo(os.Stdout, value)
}

// PrintTo writes value to w as two-space indented JSON. `<`, `>` and `&`
// stay literal (no \u003c): the reader is a terminal or a JSON parser, never
// an HTML page.
func PrintTo(w io.Writer, value any) error {
	if file, ok := w.(*os.File); ok && file == os.Stdout {
		stdoutDocument.Store(true)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

// WroteStdoutDocument reports whether a JSON document was written to
// os.Stdout since the last ResetStdoutDocument.
func WroteStdoutDocument() bool { return stdoutDocument.Load() }

// ResetStdoutDocument starts a new command run.
func ResetStdoutDocument() { stdoutDocument.Store(false) }

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

// PrintResultError prints a failed command's result WithError and returns err
// marked as reported, so `--json 2>&1` sees one document that names the
// failure. A nil err prints value unchanged.
func PrintResultError(w io.Writer, value any, err error) error {
	if err == nil {
		return PrintTo(w, value)
	}
	if printErr := PrintTo(w, WithError(value, err)); printErr != nil {
		return printErr
	}
	return Reported(err)
}

// WithError returns value's JSON object with err in its "error" field (an
// existing non-empty "error" is kept) and "ok" defaulting to false.
func WithError(value any, err error) map[string]any {
	fields := map[string]any{}
	if data, marshalErr := json.Marshal(value); marshalErr == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if decoder.Decode(&fields) != nil || fields == nil {
			fields = map[string]any{}
		}
	}
	if _, ok := fields["ok"]; !ok {
		fields["ok"] = false
	}
	if existing, _ := fields["error"].(string); existing == "" && err != nil {
		fields["error"] = err.Error()
	}
	return fields
}
