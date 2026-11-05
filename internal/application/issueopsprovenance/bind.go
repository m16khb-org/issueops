// Package issueopsprovenance is the application service that binds generated
// commands to the observed provenance (executable path and digest, lease
// generation) of the binary that produced them.
package issueopsprovenance

import (
	"context"
	"fmt"

	commandparsecontract "issueops/internal/contract/commandparse"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

// Bind binds command to the executable provenance observed through observer
// and the lease generation, returning the bound command. An empty command is
// returned unchanged without observing.
func Bind(ctx context.Context, command string, generation uint64, observer provenanceport.Observer) (string, error) {
	bound, err := BindMany(ctx, []string{command}, generation, observer)
	if err != nil {
		return "", err
	}
	return bound[0], nil
}

// BindMany binds every non-empty command to one provenance observation taken
// through observer, so all commands share the same executable evidence and
// generation. Empty commands stay empty, and no observation happens if all are
// empty. A missing or failing observer yields a provenance observation error.
func BindMany(ctx context.Context, commands []string, generation uint64, observer provenanceport.Observer) ([]string, error) {
	bound := append([]string(nil), commands...)
	hasCommand := false
	for _, command := range commands {
		hasCommand = hasCommand || command != ""
	}
	if !hasCommand {
		return bound, nil
	}
	if observer == nil {
		return nil, commandparsecontract.NewGeneratedCommandProvenanceObservationError(fmt.Errorf("observer is unavailable"))
	}
	receipt, err := observer.Observe(ctx)
	if err != nil {
		return nil, commandparsecontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	evidence := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  generation,
	}
	for index, command := range commands {
		if command == "" {
			continue
		}
		bound[index], err = commandparsecontract.BindGeneratedCommand(command, evidence)
		if err != nil {
			return nil, err
		}
	}
	return bound, nil
}
