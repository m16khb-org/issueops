package trace

import (
	"bytes"
	"fmt"
	"io"
	statecontract "issueops/internal/contract/state"
	tracecontract "issueops/internal/contract/trace"
	"os"
)

type Source struct {
	ReadState func(string) (statecontract.StateResult, error)
	Stdin     io.Reader
}

func (source Source) Load(input, format string) (string, []byte, bool, error) {
	if isHostFormat(format) {
		return source.loadHost(input)
	}
	if input == "-" {
		body, err := os.ReadFile("/dev/stdin")
		return "stdin", body, false, err
	}
	if body, err := os.ReadFile(input); err == nil {
		return "file", body, false, nil
	}
	state, err := source.ReadState(input)
	if err != nil {
		return "", nil, false, fmt.Errorf("read trace input as file or state key %q: %w", input, err)
	}
	return "state", []byte(state.Record.Content), false, nil
}

func (source Source) loadHost(input string) (string, []byte, bool, error) {
	name := "file"
	var reader io.Reader
	if input == "-" {
		name = "stdin"
		reader = source.Stdin
		if reader == nil {
			reader = os.Stdin
		}
	} else {
		file, err := os.Open(input)
		if err != nil {
			return "", nil, false, fmt.Errorf("read host trace input: %w", err)
		}
		defer file.Close()
		reader = file
	}
	body, err := io.ReadAll(io.LimitReader(reader, tracecontract.HostInputMaxBytes+1))
	if err != nil {
		return name, nil, false, fmt.Errorf("read host trace input: %w", err)
	}
	if len(body) <= tracecontract.HostInputMaxBytes {
		return name, body, false, nil
	}
	body = body[:tracecontract.HostInputMaxBytes]
	if index := bytes.LastIndexByte(body, '\n'); index >= 0 {
		return name, body[:index+1], true, nil
	}
	return name, nil, true, nil
}
