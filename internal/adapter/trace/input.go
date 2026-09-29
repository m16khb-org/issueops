package trace

import (
	"fmt"
	statecontract "issueops/internal/contract/state"
	"os"
)

type Source struct {
	ReadState func(string) (statecontract.StateResult, error)
}

func (source Source) Load(input string) (string, []byte, error) {
	if input == "-" {
		body, err := os.ReadFile("/dev/stdin")
		return "stdin", body, err
	}
	if body, err := os.ReadFile(input); err == nil {
		return "file", body, nil
	}
	state, err := source.ReadState(input)
	if err != nil {
		return "", nil, fmt.Errorf("read trace input as file or state key %q: %w", input, err)
	}
	return "state", []byte(state.Record.Content), nil
}
