package qualitycli

import (
	"io"
	contract "issueops/internal/contract/quality"
)

type Deps struct {
	Root            string
	Inspect         func(string) contract.InspectResult
	ReadSNRBaseline func(string) (float64, bool, error)
	SaveSNRBaseline func(string, float64) error
	PrintJSON       func(any) error
	Output          io.Writer
}
