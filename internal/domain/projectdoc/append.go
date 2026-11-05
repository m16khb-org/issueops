package projectdoc

import (
	"fmt"
	"strings"
)

// AppendInput is the request to append a dated record: Kind selects the record family (caution or adr, with aliases), Title and Summary describe it.
type AppendInput struct{ Kind, Title, Summary string }

// AppendPlan is the validated placement of a new record: the canonical Kind ("caution" or "adr"), the module directory under the project docs directory, and the file-name Slug derived from the title.
type AppendPlan struct{ Kind, ModuleDir, Slug string }

// PlanAppend validates input and decides where the new record belongs. It
// folds Kind aliases ("failure", "decision", ...) to "caution" or "adr", and
// returns an error for an unsupported kind or an empty title or summary. It
// does no I/O; the caller writes the file.
func PlanAppend(input AppendInput) (AppendPlan, error) {
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	kind = strings.ReplaceAll(kind, "_", "-")
	kind = strings.ReplaceAll(kind, " ", "-")
	switch kind {
	case "caution", "cautions", "false-case", "failure", "problem":
		kind = "caution"
	case "adr", "decision", "architecture-decision":
		kind = "adr"
	default:
		return AppendPlan{}, fmt.Errorf("unsupported record kind %q: use caution or adr", input.Kind)
	}
	if strings.TrimSpace(input.Title) == "" {
		return AppendPlan{}, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(input.Summary) == "" {
		return AppendPlan{}, fmt.Errorf("summary is required")
	}
	moduleDir := "cautions"
	if kind == "adr" {
		moduleDir = "adr"
	}
	return AppendPlan{Kind: kind, ModuleDir: moduleDir, Slug: recordSlug(input.Title)}, nil
}

func recordSlug(title string) string {
	var builder strings.Builder
	lastHyphen := false
	for _, char := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastHyphen = false
		case !lastHyphen && builder.Len() > 0:
			builder.WriteByte('-')
			lastHyphen = true
		}
		if builder.Len() >= 60 {
			break
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "record"
	}
	return slug
}
