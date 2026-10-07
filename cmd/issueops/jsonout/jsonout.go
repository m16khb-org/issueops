// Package jsonout writes the indented JSON every issueops CLI command prints.
// It is a leaf: it imports no other cmd package, so any command package and
// the issueopsapp composition root can share it without an import cycle.
package jsonout

import (
	"encoding/json"
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
