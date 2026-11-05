// Package qualitycli implements the `issueops quality` CLI group. It runs the quality
// inspection through injected effects and prints the result as JSON or text.
package qualitycli

import (
	"io"
	contract "issueops/internal/contract/quality"
)

// Deps holds the effects `quality inspect` needs: the default repository Root, the
// inspection function, SNR baseline read/save hooks, the JSON printer, and the
// text Output writer (os.Stdout when nil).
type Deps struct {
	Root            string
	Inspect         func(string) contract.InspectResult
	ReadSNRBaseline func(string) (float64, bool, error)
	SaveSNRBaseline func(string, float64) error
	PrintJSON       func(any) error
	Output          io.Writer
}
