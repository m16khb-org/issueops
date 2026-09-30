package issueopsremote

import (
	"encoding/json"
	"fmt"
	"strings"

	reportcontract "issueops/internal/contract/artifactreadability"
	"issueops/internal/domain/artifactreadability"
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
	Confirm   bool
}

// ResolvedTemplateBody is the body a create command publishes and the
// readability report that judged it.
type ResolvedTemplateBody struct {
	Body        string
	Readability reportcontract.Report
}

// resolveTemplateBody renders the body against its contract and always runs
// the readability check. A preview reports the findings; a confirm refuses
// any critical finding before anything is sealed or sent to the provider.
// create-issue has four templates, so its confirm must name one; create-child
// and create-pr fall back to their single template.
func (s TemplateBodyResolver) Resolve(req TemplateBodyRequest) (ResolvedTemplateBody, error) {
	body, err := s.ReadBody(req.Body, req.BodyFile)
	if err != nil {
		return ResolvedTemplateBody{}, err
	}
	template := strings.TrimSpace(req.Template)
	if template == "" && req.Confirm && req.Kind == artifacttemplate.IssueOpsArtifactIssue {
		return ResolvedTemplateBody{}, fmt.Errorf("--template is required with --confirm: pass bug, feature, proposal, or implementation_task")
	}
	fields, err := artifacttemplate.ParseFieldAssignments(req.Fields)
	if err != nil {
		return ResolvedTemplateBody{}, err
	}
	scoreSummary, err := s.ScoreSummary(req.ScoreFile)
	if err != nil {
		return ResolvedTemplateBody{}, err
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
	if body == "" && len(fields) == 0 {
		// Nothing was authored: judge the empty body instead of publishing an
		// empty skeleton. The skeleton belongs to render-template.
		result.Body = ""
	}
	var inputErrors []string
	for _, code := range result.Validation.Critical {
		if !readabilityDeferredCodes[code] {
			inputErrors = append(inputErrors, code)
		}
	}
	if len(inputErrors) > 0 {
		return ResolvedTemplateBody{}, fmt.Errorf("template validation failed: %s", strings.Join(inputErrors, ","))
	}
	resolved := ResolvedTemplateBody{
		Body: result.Body,
		Readability: artifactreadability.Check(artifactreadability.Input{
			Kind:     artifactreadability.KindFor(result.Kind),
			Template: result.Template,
			Title:    req.Title,
			Body:     result.Body,
			Fields:   fields,
		}),
	}
	if req.Confirm && !resolved.Readability.OK {
		return resolved, artifactreadability.RefusalError(resolved.Readability)
	}
	return resolved, nil
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

func (s TemplateBodyResolver) ReadBody(body, bodyFile string) (string, error) {
	body = strings.TrimSpace(body)
	bodyFile = strings.TrimSpace(bodyFile)
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("body and body-file are mutually exclusive")
	}
	if bodyFile == "" {
		return body, nil
	}
	b, err := s.readFile(bodyFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

var readabilityDeferredCodes = map[string]bool{
	"summary_section_missing": true, "required_section_missing": true,
	"placeholder_section": true, "missing_required_fields": true, "korean_body_required": true,
}
