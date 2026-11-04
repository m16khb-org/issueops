// Package trace reads and decodes trace input from files, stdin, state
// records, and host usage logs into the domain trace model. It is an
// adapter-layer package.
package trace

import (
	"bytes"
	"fmt"
	"io"
	statecontract "issueops/internal/contract/state"
	tracecontract "issueops/internal/contract/trace"
	"os"
	"strings"
)

// Source loads raw trace input. ReadState reads a state record by key; Stdin,
// when set, replaces os.Stdin for host-format input.
type Source struct {
	ReadState func(string) (statecontract.Result, error)
	Stdin     io.Reader
}

// Load reads trace input and returns where it came from ("stdin", "file", or
// "state"), its bytes, and whether host input was truncated at the size limit.
// Host formats read from a file or stdin. Otherwise input "-" is stdin, an
// existing file is read, and a bare name without a path separator is looked up
// as a state key.
func (source Source) Load(input, format string) (string, []byte, bool, error) {
	if isHostFormat(format) {
		return source.loadHost(input)
	}
	if input == "-" {
		body, err := os.ReadFile("/dev/stdin")
		return "stdin", body, false, err
	}
	body, fileErr := os.ReadFile(input)
	if fileErr == nil {
		return "file", body, false, nil
	}
	if strings.ContainsAny(input, `/\`) {
		// Probe: a missing path fell through to the state lookup and
		// answered "invalid state key"; a path is never a state key.
		return "", nil, false, fileErr
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
