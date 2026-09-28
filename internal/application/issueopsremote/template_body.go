package issueopsremote

import (
	"encoding/json"
	"fmt"
	"strings"

	artifacttemplate "issueops/internal/domain/artifacttemplate"
	issueopsremote "issueops/internal/domain/issueopsremote"
)

type TemplateBodyResolver struct{ readFile func(string) ([]byte, error) }

func NewTemplateBodyResolver(readFile func(string) ([]byte, error)) TemplateBodyResolver {
	return TemplateBodyResolver{readFile: readFile}
}

type TemplateBodyRequest struct {
	Kind      artifacttemplate.IssueOpsArtifactKind
	Template  string
	Provider  string
	Title     string
	Body      string
	BodyFile  string
	Fields    []string
	ScoreFile string
}

func (s TemplateBodyResolver) Resolve(req TemplateBodyRequest) (string, error) {
	body := strings.TrimSpace(req.Body)
	bodyFile := strings.TrimSpace(req.BodyFile)
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("body and body-file are mutually exclusive")
	}
	if bodyFile != "" {
		b, err := s.readFile(bodyFile)
		if err != nil {
			return "", err
		}
		body = strings.TrimSpace(string(b))
	}
	template := strings.TrimSpace(req.Template)
	if template == "" {
		return body, nil
	}
	fields, err := artifacttemplate.ParseFieldAssignments(req.Fields)
	if err != nil {
		return "", err
	}
	scoreSummary, err := s.ScoreSummary(req.ScoreFile)
	if err != nil {
		return "", err
	}
	input := artifacttemplate.IssueOpsTemplateInput{
		Kind:         req.Kind,
		Template:     artifacttemplate.IssueOpsTemplateKind(template),
		Provider:     req.Provider,
		Title:        req.Title,
		Body:         body,
		Fields:       fields,
		ScoreSummary: scoreSummary,
	}
	result := artifacttemplate.Render(input)
	if len(result.Validation.Critical) > 0 {
		return "", fmt.Errorf("template validation failed: %s", strings.Join(result.Validation.Critical, ","))
	}
	return result.Body, nil
}

func (s TemplateBodyResolver) ScoreSummary(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	b, err := s.readFile(path)
	if err != nil {
		return "", err
	}
	var result issueopsremote.IssueOpsRemoteScoringResult
	if err := json.Unmarshal(b, &result); err != nil {
		return strings.TrimSpace(string(b)), nil
	}
	parts := []string{fmt.Sprintf("threshold %.2f", result.Threshold)}
	if len(result.SelectedRelatedIssues) > 0 {
		parts = append(parts, "선택 관련 이슈: "+joinScoredItems(result.SelectedRelatedIssues))
	}
	if len(result.RejectedRelatedIssues) > 0 {
		parts = append(parts, "거절 관련 이슈: "+joinScoredItems(result.RejectedRelatedIssues))
	}
	if len(result.SelectedLabels) > 0 {
		parts = append(parts, "선택 라벨: "+joinScoredItems(result.SelectedLabels))
	}
	if len(result.RejectedLabels) > 0 {
		parts = append(parts, "거절 라벨: "+joinScoredItems(result.RejectedLabels))
	}
	return strings.Join(parts, "\n"), nil
}

func joinScoredItems(items []issueopsremote.IssueOpsRemoteScoredItem) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := firstNonEmpty(item.Name, item.ID, item.Title, item.URL)
		if name == "" {
			name = "unknown"
		}
		out = append(out, fmt.Sprintf("%s(%.2f)", name, item.Score))
	}
	return strings.Join(out, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
