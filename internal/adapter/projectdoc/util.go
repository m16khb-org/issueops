package projectdoc

import (
	"os"
)

// PlannedFileAction classifies what writing content to path would do: "create"
// when path cannot be read, "unchanged" when it already holds content, and
// "update" otherwise.
func PlannedFileAction(path, content string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "create"
	}
	if string(b) == content {
		return "unchanged"
	}
	return "update"
}
