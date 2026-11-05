package issueopsremote

import (
	"encoding/json"
	"fmt"
	"strings"

	reportcontract "issueops/internal/contract/artifactreadability"
	"issueops/internal/domain/artifactreadability"
	artifacttemplate "issueops/internal/domain/artifacttemplate"
	"issueops/internal/domain/issueopsremote"
)

// TemplateBodyResolver 타입은 직접 입력, 본문 파일, 템플릿으로 생성 명령의 본문을 만든다.
type TemplateBodyResolver struct{ readFile func(string) ([]byte, error) }

// NewTemplateBodyResolver 함수는 본문·점수 파일을 읽는 함수로 TemplateBodyResolver를 만든다.
func NewTemplateBodyResolver(readFile func(string) ([]byte, error)) TemplateBodyResolver {
	return TemplateBodyResolver{readFile: readFile}
}

type templateBodyRequest struct {
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

// Resolve 메서드는 본문을 계약에 맞춰 렌더링하고 항상 가독성 검사를 실행한다.
// 미리보기는 발견 사항을 보고하고, confirm은 critical 발견이 있으면 봉인하거나 provider에 보내기 전에 거부한다.
// create-issue는 템플릿이 네 개라 confirm에서 하나를 지정해야 하고,
// create-child와 create-pr는 단일 템플릿을 기본으로 쓴다.
func (s TemplateBodyResolver) Resolve(req templateBodyRequest) (ResolvedTemplateBody, error) {
	body, err := s.readBody(req.Body, req.BodyFile)
	if err != nil {
		return ResolvedTemplateBody{}, err
	}
	template := strings.TrimSpace(req.Template)
	templateMissing := template == "" && req.Kind == artifacttemplate.IssueOpsArtifactIssue
	if templateMissing && req.Confirm {
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
	result := renderTemplateBody(req, template, body, fields, scoreSummary)
	if inputErrors := blockingInputErrors(result.Validation.Critical); len(inputErrors) > 0 {
		return ResolvedTemplateBody{}, templateValidationError(req.Kind, template, inputErrors)
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
	if templateMissing {
		// Without a template the preview cannot check the sections the
		// confirmed request will require, so a clean preview would be a
		// false pass: say so where the preview reports readability.
		resolved.Readability.OK = false
		resolved.Readability.Critical = append(resolved.Readability.Critical, reportcontract.Finding{
			Code:    "template_missing",
			Message: "--confirm requires --template bug|feature|proposal|implementation_task; preview with it to check that template's required sections",
		})
	}
	if req.Confirm && !resolved.Readability.OK {
		return resolved, artifactreadability.RefusalError(resolved.Readability)
	}
	return resolved, nil
}

func renderTemplateBody(req templateBodyRequest, template, body string, fields map[string]string, scoreSummary string) artifacttemplate.IssueOpsTemplateResult {
	result := artifacttemplate.Render(artifacttemplate.IssueOpsTemplateInput{
		Kind:         req.Kind,
		Template:     artifacttemplate.IssueOpsTemplateKind(template),
		Provider:     req.Provider,
		Title:        req.Title,
		Body:         body,
		Fields:       fields,
		ScoreSummary: scoreSummary,
	})
	if body == "" && len(fields) == 0 {
		// Nothing was authored: judge the empty body instead of publishing an
		// empty skeleton. The skeleton belongs to render-template.
		result.Body = ""
	}
	return result
}

// blockingInputErrors drops the codes the readability check judges instead.
func blockingInputErrors(critical []string) []string {
	var inputErrors []string
	for _, code := range critical {
		if !readabilityDeferredCodes[code] {
			inputErrors = append(inputErrors, code)
		}
	}
	return inputErrors
}

func templateValidationError(kind artifacttemplate.IssueOpsArtifactKind, template string, inputErrors []string) error {
	if choice := artifacttemplate.ChoiceError(string(kind), template); choice != nil {
		// Probe: --template bogus answered the bare code.
		return fmt.Errorf("template validation failed: %s: %w", strings.Join(inputErrors, ","), choice)
	}
	return fmt.Errorf("template validation failed: %s", strings.Join(inputErrors, ","))
}

// ScoreSummary 메서드는 점수 파일을 읽어 요약 문자열로 만든다. 경로가 비면 빈 문자열이고, JSON이 아니면 파일 내용을 그대로 쓴다.
func (s TemplateBodyResolver) ScoreSummary(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	b, err := s.readFile(path)
	if err != nil {
		return "", err
	}
	var result issueopsremote.ScoringResult
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

func (s TemplateBodyResolver) readBody(body, bodyFile string) (string, error) {
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
