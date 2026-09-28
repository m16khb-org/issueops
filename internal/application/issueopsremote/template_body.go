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
	return issueopsremote.RenderScoreSummary(result), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
