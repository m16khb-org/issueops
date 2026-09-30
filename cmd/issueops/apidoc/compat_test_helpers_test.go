package apidoc

type apiDocStaticViolation = StaticViolation

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return CheckNestDTOStatic(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt, evidence string) string {
	return BuildReviewPrompt(files, diff, extraPrompt, evidence)
}

func apiDocReviewSchema() map[string]any {
	return ReviewSchema()
}

func apiDocReviewExtraPrompt(options apiDocReviewOptions) (string, error) {
	return ReviewExtraPrompt(options.Repo, options.PromptFile)
}

func apiDocDiff(repo string, files []string, diffFile string) (string, error) {
	return Diff(repo, files, diffFile)
}

func apiDocInput(repo string, files []string, diffFile string, all bool) (string, error) {
	return Input(repo, files, diffFile, all)
}

func trackedAPIDocFiles(repo string) []string {
	return TrackedFiles(repo)
}

func isAPIDocCandidate(file string) bool {
	return IsCandidate(file)
}
