package apidoc

import domain "issueops/internal/domain/apidoc"

type ReviewFileEffects struct {
	ExtraPrompt func(string, string) (string, error)
	Diff        func(string, []string, string) (string, error)
	Input       func(string, []string, string, bool) (string, error)
	FullContent func(string, []string) (string, error)
	Staged      func(string) []string
	Tracked     func(string) []string
	Normalize   func(string, []string) []string
	Evidence    func(string, []string) string
}

var reviewFileEffects ReviewFileEffects

func ConfigureReviewFiles(effects ReviewFileEffects) { reviewFileEffects = effects }

func ReviewExtraPrompt(repo, promptFile string) (string, error) {
	return reviewFileEffects.ExtraPrompt(repo, promptFile)
}
func Diff(repo string, files []string, diffFile string) (string, error) {
	return reviewFileEffects.Diff(repo, files, diffFile)
}
func Input(repo string, files []string, diffFile string, all bool) (string, error) {
	return reviewFileEffects.Input(repo, files, diffFile, all)
}
func FullContent(repo string, files []string) (string, error) {
	return reviewFileEffects.FullContent(repo, files)
}
func StagedFiles(repo string) []string  { return reviewFileEffects.Staged(repo) }
func TrackedFiles(repo string) []string { return reviewFileEffects.Tracked(repo) }
func NormalizeFiles(repo string, files []string) []string {
	return reviewFileEffects.Normalize(repo, files)
}
func IsCandidate(file string) bool { return domain.IsCandidate(file) }
