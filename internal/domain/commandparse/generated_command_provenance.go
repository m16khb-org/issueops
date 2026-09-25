package commandparse

import commandparsecontract "issueops/internal/contract/commandparse"

// ValidateGeneratedCommandInvocation requires a generated command to match both
// the durable lease generation and the executable that is running it.
func ValidateGeneratedCommandInvocation(expected, observed commandparsecontract.GeneratedCommandProvenance, durableGeneration uint64) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	if err := observed.Validate(); err != nil {
		return err
	}
	if expected.LeaseGeneration != durableGeneration || observed.LeaseGeneration != durableGeneration {
		return &commandparsecontract.GeneratedCommandProvenanceError{
			Code: "generated_command_generation_mismatch", Message: "generated command lease generation does not match durable state",
			Expected: expected, Observed: observed, Generation: durableGeneration,
		}
	}
	if expected.ExecutablePath != observed.ExecutablePath || expected.ExecutableSHA256 != observed.ExecutableSHA256 {
		return &commandparsecontract.GeneratedCommandProvenanceError{
			Code: "generated_command_binary_provenance_mismatch", Message: "generated command binary provenance does not match the executing binary",
			Expected: expected, Observed: observed, Generation: durableGeneration,
		}
	}
	return nil
}
