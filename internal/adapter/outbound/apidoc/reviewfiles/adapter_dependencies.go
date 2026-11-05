// Package reviewfiles is the outbound adapter that supplies the API documentation
// review with its file-system and git inputs: candidate file selection, diff or
// full-content review input, repository spec mode, and extra prompt text.
package reviewfiles

// Files lists and reads the files an API documentation review covers. GitCmd runs
// git in the given directory and returns its exit code, stdout, and stderr.
type Files struct {
	GitCmd func(dir string, args ...string) (int, string, string)
}
