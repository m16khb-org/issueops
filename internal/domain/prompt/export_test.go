package prompt

// StructuredPromptSectionHeadings lists the fixed sections BuildStructuredPrompt
// emits, in order. It is exported only to tests.
var StructuredPromptSectionHeadings = []string{
	headingIdentity,
	headingObjective,
	headingOperatingPhases,
	headingInputs,
	headingRules,
	headingOutputContract,
	headingVerificationChecklist,
}
