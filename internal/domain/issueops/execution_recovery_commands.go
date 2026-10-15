package issueops

import "strconv"

func ReplacementPreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
}

func ReplacementFinalizePreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --finalize-preview"
}
