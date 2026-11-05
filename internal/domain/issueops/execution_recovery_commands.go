package issueops

import "strconv"

// ReplacementPreviewCommand returns the exact command that previews replacing
// the execution lease of cycle id at the expected generation. The ID is
// shell-quoted.
func ReplacementPreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
}

// ReplacementFinalizePreviewCommand returns the exact command that previews
// finalizing a replacement of the execution lease of cycle id at the expected
// generation. The ID is shell-quoted.
func ReplacementFinalizePreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --finalize-preview"
}
