package artifactreadability

import (
	"fmt"
	"strings"

	reportcontract "issueops/internal/contract/artifactreadability"
	"issueops/internal/domain/artifacttemplate"
)

func BodySyncArtifactKind(kind string) artifacttemplate.IssueOpsArtifactKind {
	switch kind {
	case "child":
		return artifacttemplate.IssueOpsArtifactChild
	case "pr", "mr":
		return artifacttemplate.IssueOpsArtifactPR
	default:
		return artifacttemplate.IssueOpsArtifactIssue
	}
}

// BodySyncTemplate resolves the contract a proposal is judged against. The
// template is not stored in the record (its decoder rejects unknown fields),
// so without --template it is inferred from the proposal's own sections.
func BodySyncTemplate(kind artifacttemplate.IssueOpsArtifactKind, template, body string) (artifacttemplate.IssueOpsTemplateKind, error) {
	named := artifacttemplate.IssueOpsTemplateKind(strings.ToLower(strings.TrimSpace(template)))
	if named == "" {
		return artifacttemplate.InferTemplateKind(kind, body), nil
	}
	if !artifacttemplate.SupportsTemplate(kind, named) {
		return "", fmt.Errorf("template %q does not apply to a %s body", named, kind)
	}
	return named, nil
}

func CheckBodySyncReadability(kind artifacttemplate.IssueOpsArtifactKind, template artifacttemplate.IssueOpsTemplateKind, body string) reportcontract.Report {
	return Check(Input{Kind: KindFor(kind), Template: template, Body: body})
}

// WarningOnly moves every critical finding to the warnings.
func WarningOnly(report reportcontract.Report) reportcontract.Report {
	report.Warnings = append(append([]reportcontract.Finding{}, report.Critical...), report.Warnings...)
	report.Critical = []reportcontract.Finding{}
	report.OK = true
	return report
}
