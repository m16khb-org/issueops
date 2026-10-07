package artifacttemplate

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

type IssueOpsArtifactKind string

const (
	IssueOpsArtifactIssue IssueOpsArtifactKind = "issue"
	IssueOpsArtifactChild IssueOpsArtifactKind = "child"
	IssueOpsArtifactPR    IssueOpsArtifactKind = "pr"
)

type IssueOpsTemplateKind string

const (
	IssueOpsTemplateBug                IssueOpsTemplateKind = "bug"
	IssueOpsTemplateFeature            IssueOpsTemplateKind = "feature"
	IssueOpsTemplateProposal           IssueOpsTemplateKind = "proposal"
	IssueOpsTemplateImplementationTask IssueOpsTemplateKind = "implementation_task"
	IssueOpsTemplateChildTask          IssueOpsTemplateKind = "child_task"
	IssueOpsTemplatePullRequest        IssueOpsTemplateKind = "pull_request"
)

// SummaryHeading is the canonical first section every rendered body must
// carry. Structural validation (summary_section_missing) checks for this
// exact heading.
const SummaryHeading = "## 요약"

type IssueOpsTemplateInput struct {
	Kind         IssueOpsArtifactKind `json:"kind"`
	Template     IssueOpsTemplateKind `json:"template"`
	Provider     string               `json:"provider,omitempty"`
	Title        string               `json:"title"`
	Body         string               `json:"body,omitempty"`
	Fields       map[string]string    `json:"fields,omitempty"`
	ScoreSummary string               `json:"score_summary,omitempty"`
}

type IssueOpsTemplateValidation struct {
	OK                      bool     `json:"ok"`
	Critical                []string `json:"critical"`
	Warnings                []string `json:"warnings"`
	MissingRequiredFields   []string `json:"missing_required_fields"`
	MissingRequiredSections []string `json:"missing_required_sections,omitempty"`
}

type IssueOpsTemplateResult struct {
	OK                    bool                       `json:"ok"`
	Kind                  IssueOpsArtifactKind       `json:"kind"`
	Template              IssueOpsTemplateKind       `json:"template"`
	Provider              string                     `json:"provider,omitempty"`
	Title                 string                     `json:"title"`
	Body                  string                     `json:"body"`
	Warnings              []string                   `json:"warnings"`
	MissingRequiredFields []string                   `json:"missing_required_fields"`
	Validation            IssueOpsTemplateValidation `json:"validation"`
}

func Render(input IssueOpsTemplateInput) IssueOpsTemplateResult {
	input = normalizeInput(input)
	originalBodyEmpty := strings.TrimSpace(input.Body) == ""
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = renderBody(input)
	}
	input.Body = body
	validation := Validate(input)
	if originalBodyEmpty {
		validation.MissingRequiredFields = uniqueSorted(missingFields(IssueOpsTemplateInput{
			Kind:     input.Kind,
			Template: input.Template,
			Provider: input.Provider,
			Title:    input.Title,
			Fields:   input.Fields,
		}))
		if len(validation.MissingRequiredFields) > 0 && !slices.Contains(validation.Critical, "missing_required_fields") {
			validation.Critical = uniqueSorted(append(validation.Critical, "missing_required_fields"))
		}
		validation.OK = len(validation.Critical) == 0
	}
	return IssueOpsTemplateResult{
		OK:                    validation.OK,
		Kind:                  input.Kind,
		Template:              input.Template,
		Provider:              input.Provider,
		Title:                 strings.TrimSpace(input.Title),
		Body:                  body,
		Warnings:              validation.Warnings,
		MissingRequiredFields: validation.MissingRequiredFields,
		Validation:            validation,
	}
}

func Validate(input IssueOpsTemplateInput) IssueOpsTemplateValidation {
	input = normalizeInput(input)
	v := IssueOpsTemplateValidation{OK: true, Critical: []string{}, Warnings: []string{}, MissingRequiredFields: []string{}, MissingRequiredSections: []string{}}
	if !supportedArtifactKind(input.Kind) {
		v.Critical = append(v.Critical, "unsupported_artifact_kind")
	}
	if !SupportsTemplate(input.Kind, input.Template) {
		v.Critical = append(v.Critical, "unsupported_template_for_artifact")
	}
	if strings.TrimSpace(input.Title) == "" {
		v.MissingRequiredFields = append(v.MissingRequiredFields, "title")
	}
	v.MissingRequiredFields = append(v.MissingRequiredFields, missingFields(input)...)
	body := strings.TrimSpace(input.Body)
	if strings.Contains(strings.ToLower(body), "## plan link") || strings.Contains(strings.ToLower(body), "## plan") {
		v.Critical = append(v.Critical, "plan_link_section_forbidden")
	}
	if strings.EqualFold(input.Provider, "gitlab") && strings.Contains(strings.ToLower(body), "## related issues") {
		v.Critical = append(v.Critical, "gitlab_related_issues_body_section_forbidden")
	}
	if body != "" && !containsHangul(body) {
		v.Critical = append(v.Critical, "korean_body_required")
	}
	if body != "" && supportedArtifactKind(input.Kind) && SupportsTemplate(input.Kind, input.Template) {
		if !hasNonEmptySummarySection(body) {
			v.Critical = append(v.Critical, "summary_section_missing")
		}
		for _, spec := range sectionsFor(input.Kind, input.Template) {
			if !spec.Required {
				continue
			}
			content, ok := sectionContentFromBody(body, spec.Title)
			if !ok {
				v.MissingRequiredSections = append(v.MissingRequiredSections, spec.Title)
				continue
			}
			if isPlaceholderContent(content) {
				v.Critical = append(v.Critical, "placeholder_section")
			}
		}
		if len(v.MissingRequiredSections) > 0 {
			v.Critical = append(v.Critical, "required_section_missing")
		}
	}
	for _, key := range unrenderedFields(input) {
		v.Warnings = append(v.Warnings, "unrendered_field:"+key)
	}
	if len(v.MissingRequiredFields) > 0 {
		v.Critical = append(v.Critical, "missing_required_fields")
	}
	if input.Provider == "" {
		v.Warnings = append(v.Warnings, "provider_not_set")
	}
	v.MissingRequiredFields = uniqueSorted(v.MissingRequiredFields)
	// MissingRequiredSections stays in contract order so a reader sees the
	// gaps in the order the body should hold them.
	v.Critical = uniqueSorted(v.Critical)
	v.Warnings = uniqueSorted(v.Warnings)
	v.OK = len(v.Critical) == 0
	return v
}

func supportedArtifactKind(kind IssueOpsArtifactKind) bool {
	switch kind {
	case IssueOpsArtifactIssue, IssueOpsArtifactChild, IssueOpsArtifactPR:
		return true
	default:
		return false
	}
}

// SupportsTemplate reports whether template is one of kind's body contracts.
func SupportsTemplate(kind IssueOpsArtifactKind, template IssueOpsTemplateKind) bool {
	switch kind {
	case IssueOpsArtifactIssue:
		switch template {
		case IssueOpsTemplateBug, IssueOpsTemplateFeature, IssueOpsTemplateProposal, IssueOpsTemplateImplementationTask:
			return true
		}
	case IssueOpsArtifactChild:
		return template == IssueOpsTemplateChildTask
	case IssueOpsArtifactPR:
		return template == IssueOpsTemplatePullRequest
	}
	return false
}

func normalizeInput(input IssueOpsTemplateInput) IssueOpsTemplateInput {
	input.Kind = IssueOpsArtifactKind(strings.ToLower(strings.TrimSpace(string(input.Kind))))
	input.Template = IssueOpsTemplateKind(strings.ToLower(strings.TrimSpace(string(input.Template))))
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if input.Fields == nil {
		input.Fields = map[string]string{}
	}
	input.Fields = normalizeFields(input.Fields)
	if input.Kind == "" {
		input.Kind = IssueOpsArtifactIssue
	}
	if input.Template == "" {
		switch input.Kind {
		case IssueOpsArtifactChild:
			input.Template = IssueOpsTemplateChildTask
		case IssueOpsArtifactPR:
			input.Template = IssueOpsTemplatePullRequest
		default:
			input.Template = IssueOpsTemplateImplementationTask
		}
	}
	return input
}

// unrenderedFields lists the non-empty fields that no section of the
// kind/template renders. They are accepted (never rejected), and Validate
// warns `unrendered_field:<key>` for each so authors know it was dropped.
func unrenderedFields(input IssueOpsTemplateInput) []string {
	rendered := map[string]bool{}
	for _, spec := range sectionsFor(input.Kind, input.Template) {
		for _, key := range sectionFieldNames(spec.Key) {
			rendered[key] = true
		}
	}
	var keys []string
	for key, value := range input.Fields {
		if !rendered[key] && strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func normalizeFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(fields))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		normalized := normalizeFieldKey(key)
		value := strings.TrimSpace(fields[key])
		if value == "" {
			if _, exists := out[normalized]; !exists {
				out[normalized] = ""
			}
			continue
		}
		if existing := strings.TrimSpace(out[normalized]); existing != "" {
			out[normalized] = existing + "\n\n" + value
			continue
		}
		out[normalized] = value
	}
	return out
}

func normalizeFieldKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	return key
}

func renderBody(input IssueOpsTemplateInput) string {
	sections := []section{}
	for _, spec := range sectionsFor(input.Kind, input.Template) {
		content := combinedField(input, spec.Key)
		if !spec.Required && content == "" {
			continue
		}
		sections = append(sections, section{spec.Title, content})
	}
	return renderSections(sections)
}

type section struct {
	title string
	body  string
}

func renderSections(sections []section) string {
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(section.title)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(section.body))
	}
	return strings.TrimSpace(b.String())
}

func field(input IssueOpsTemplateInput, key string) string {
	return strings.TrimSpace(input.Fields[key])
}

// sectionSpec describes one canonical section of the reader-first body
// contract: its rendered title, the field key that carries its content, and
// whether the contract requires it for the given kind/template.
type sectionSpec struct {
	Key      string
	Title    string
	Required bool
}

// sectionsFor is the single source of truth for the body contract: which
// sections a kind/template combination renders, in what order, and which are
// required. Structural validation (Validate) and rendering (renderBody) both
// read from here so the two can never drift apart.
func sectionsFor(kind IssueOpsArtifactKind, template IssueOpsTemplateKind) []sectionSpec {
	switch kind {
	case IssueOpsArtifactChild:
		return []sectionSpec{
			{"summary", "요약", true},
			{"acceptance", "완료 기준", true},
			{"scope", "범위", true},
			{"merge_condition", "선행 조건과 병합 조건", true},
			{"verification", "검증", false},
		}
	case IssueOpsArtifactPR:
		return []sectionSpec{
			{"summary", "요약", true},
			{"changes", "변경 내용", true},
			{"verified", "확인한 것", true},
			{"reviewer_focus", "리뷰 포인트", true},
			{"alternatives_rationale", "대안과 선택 이유", false},
			{"risk_rollback", "위험과 되돌리기", false},
			{"compatibility_migration", "호환성과 마이그레이션", false},
			{"remaining_work", "남은 일", false},
		}
	default: // IssueOpsArtifactIssue
		switch template {
		case IssueOpsTemplateBug:
			return []sectionSpec{
				{"summary", "요약", true},
				{"reproduction_steps", "재현 절차", true},
				{"expected_actual", "기대 동작과 실제 동작", true},
				{"acceptance", "완료 기준", true},
				{"verification", "검증", true},
				{"cause", "원인", false},
				{"scope", "범위", false},
				{"environment_logs", "환경과 로그", false},
			}
		case IssueOpsTemplateProposal:
			return []sectionSpec{
				{"summary", "요약", true},
				{"background", "배경", true},
				{"proposal", "제안", true},
				{"alternatives_rationale", "대안과 선택 이유", true},
				{"scope", "범위", true},
				{"acceptance", "완료 기준", false},
				{"risks", "위험", false},
				{"open_questions", "열린 질문", false},
			}
		default: // implementation_task, feature
			return []sectionSpec{
				{"summary", "요약", true},
				{"background", "배경", true},
				{"acceptance", "완료 기준", true},
				{"scope", "범위", true},
				{"verification", "검증", true},
				{"approach_alternatives", "접근 방법과 대안", false},
				{"risks", "위험", false},
				{"open_questions", "열린 질문", false},
				{"subtasks", "하위 Task", false},
			}
		}
	}
}

// combinedField resolves a section's content. Most sections map to a single
// field key, but bug's 기대 동작과 실제 동작 and 환경과 로그 each fold two
// fields (expected_behavior/actual_behavior, environment/logs) into one
// rendered section.
func combinedField(input IssueOpsTemplateInput, key string) string {
	switch key {
	case "expected_actual":
		var parts []string
		if v := field(input, "expected_behavior"); v != "" {
			parts = append(parts, "기대: "+v)
		}
		if v := field(input, "actual_behavior"); v != "" {
			parts = append(parts, "실제: "+v)
		}
		return strings.Join(parts, "\n")
	case "environment_logs":
		var parts []string
		if v := field(input, "environment"); v != "" {
			parts = append(parts, "환경: "+v)
		}
		if v := field(input, "logs"); v != "" {
			parts = append(parts, "로그: "+v)
		}
		return strings.Join(parts, "\n")
	default:
		return field(input, key)
	}
}

func missingFields(input IssueOpsTemplateInput) []string {
	missing := []string{}
	for _, spec := range sectionsFor(input.Kind, input.Template) {
		if !spec.Required {
			continue
		}
		if combinedField(input, spec.Key) == "" && !fieldSatisfiedByBody(input.Body, spec.Key) {
			missing = append(missing, requiredFieldNames(spec.Key)...)
		}
	}
	return uniqueSorted(missing)
}

// requiredFieldNames reports the underlying --field key(s) a caller should
// supply for a combined section, so CLI error messages point at an actual
// flag instead of an internal section key.
func requiredFieldNames(sectionKey string) []string {
	if sectionKey == "expected_actual" {
		return []string{"expected_behavior", "actual_behavior"}
	}
	return []string{sectionKey}
}

// sectionFieldNames reports every --field key a section renders.
func sectionFieldNames(sectionKey string) []string {
	if sectionKey == "environment_logs" {
		return []string{"environment", "logs"}
	}
	return requiredFieldNames(sectionKey)
}

func fieldSatisfiedByBody(body, sectionKey string) bool {
	for _, spec := range allSectionSpecs() {
		if spec.Key != sectionKey {
			continue
		}
		_, ok := SectionContent(body, spec.Title)
		return ok
	}
	return false
}

// allSectionSpecs lists every section of every kind/template once, so a
// section key can be resolved to its rendered title.
func allSectionSpecs() []sectionSpec {
	var specs []sectionSpec
	for _, kt := range []struct {
		kind     IssueOpsArtifactKind
		template IssueOpsTemplateKind
	}{
		{IssueOpsArtifactIssue, IssueOpsTemplateImplementationTask},
		{IssueOpsArtifactIssue, IssueOpsTemplateBug},
		{IssueOpsArtifactIssue, IssueOpsTemplateProposal},
		{IssueOpsArtifactChild, IssueOpsTemplateChildTask},
		{IssueOpsArtifactPR, IssueOpsTemplatePullRequest},
	} {
		specs = append(specs, sectionsFor(kt.kind, kt.template)...)
	}
	return specs
}

// bodySection is one `## ` section of a markdown body.
type bodySection struct {
	title   string
	content string
}

// bodySections splits body into its `## ` sections in order. Headings inside
// fenced code blocks are content, not sections, so a body that quotes a
// markdown example cannot satisfy or break the contract with it.
func bodySections(body string) []bodySection {
	var sections []bodySection
	var content []string
	inFence := false
	flush := func() {
		if len(sections) > 0 {
			sections[len(sections)-1].content = strings.TrimSpace(strings.Join(content, "\n"))
		}
		content = nil
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if !inFence {
			if title, ok := strings.CutPrefix(trimmed, "## "); ok {
				flush()
				sections = append(sections, bodySection{title: strings.TrimSpace(title)})
				continue
			}
		}
		content = append(content, line)
	}
	flush()
	return sections
}

// SectionContent returns the text under the first `## <title>` section of
// body, up to the next section. It reports false when the section is absent.
func SectionContent(body, title string) (string, bool) {
	for _, section := range bodySections(body) {
		if section.title == title {
			return section.content, true
		}
	}
	return "", false
}

// SummarySection returns the content of body's first section when that
// section is the summary; the contract requires the summary to come first.
func SummarySection(body string) (string, bool) {
	sections := bodySections(body)
	if len(sections) == 0 || "## "+sections[0].title != SummaryHeading {
		return "", false
	}
	return sections[0].content, true
}

func hasNonEmptySummarySection(body string) bool {
	content, ok := SummarySection(body)
	return ok && content != ""
}

func sectionContentFromBody(body, title string) (string, bool) {
	if "## "+title == SummaryHeading {
		return SummarySection(body)
	}
	return SectionContent(body, title)
}

var placeholderValues = map[string]bool{
	"없음":    true,
	"해당 없음": true,
	"해당없음":  true,
	"n/a":   true,
	"na":    true,
	"tbd":   true,
	"(없음)":  true,
	"-":     true,
}

// isPlaceholderContent reports a section that says nothing: empty, or a
// single placeholder value, optionally written as one list item ("- 없음").
func isPlaceholderContent(content string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(content))
	if trimmed == "" || placeholderValues[trimmed] {
		return true
	}
	for _, marker := range []string{"- ", "* "} {
		if item, ok := strings.CutPrefix(trimmed, marker); ok && placeholderValues[strings.TrimSpace(item)] {
			return true
		}
	}
	return false
}

func containsHangul(s string) bool {
	for _, r := range s {
		if r >= '가' && r <= '힣' {
			return true
		}
	}
	return false
}

func uniqueSorted(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

// OptionalSectionTitles reports the optional section titles for kind/template,
// in contract order. artifactreadability uses this to flag an optional
// section that was rendered as a heading but left empty; it does not
// duplicate this package's judgment about which sections are optional.
func OptionalSectionTitles(kind IssueOpsArtifactKind, template IssueOpsTemplateKind) []string {
	input := normalizeInput(IssueOpsTemplateInput{Kind: kind, Template: template})
	var titles []string
	for _, spec := range sectionsFor(input.Kind, input.Template) {
		if !spec.Required {
			titles = append(titles, spec.Title)
		}
	}
	return titles
}

// InferTemplateKind guesses a body's template from its own section titles,
// for a sync that was not told --template because the remote body predates
// the reader-first contract (design section 5). Child and PR kinds have a
// single template, so only issue bodies need inference.
func InferTemplateKind(kind IssueOpsArtifactKind, body string) IssueOpsTemplateKind {
	switch kind {
	case IssueOpsArtifactChild:
		return IssueOpsTemplateChildTask
	case IssueOpsArtifactPR:
		return IssueOpsTemplatePullRequest
	}
	if _, ok := SectionContent(body, "재현 절차"); ok {
		return IssueOpsTemplateBug
	}
	if _, ok := SectionContent(body, "제안"); ok {
		return IssueOpsTemplateProposal
	}
	return IssueOpsTemplateImplementationTask
}

func ParseFieldAssignments(values []string) (map[string]string, error) {
	fields := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("field must be key=value: %s", value)
		}
		fields[key] = strings.TrimSpace(val)
	}
	return fields, nil
}
