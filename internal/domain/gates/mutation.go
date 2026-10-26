package gates

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	model "issueops/internal/contract/gates"
	"issueops/internal/domain/shelltoken"
)

func PrepareInit(req model.InitRequest) (model.InitRequest, model.InitResult, error) {
	result := model.InitResult{SchemaVersion: model.SchemaVersion}
	req.Scope = strings.TrimSpace(req.Scope)
	if req.Scope == "" {
		return req, result, fmt.Errorf("scope is required")
	}
	req.File = strings.TrimSpace(req.File)
	if req.File == "" {
		slug := gateFileSlug(req.Scope)
		if slug == "" {
			return req, result, fmt.Errorf("scope must contain a letter or number")
		}
		req.File = filepath.Join(".issueops", "gates", slug+".md")
	}
	result.File = req.File
	if len(req.Gates) == 0 {
		return req, result, fmt.Errorf("at least one --gate spec is required")
	}
	return req, result, nil
}
func RenderInitialLedger(req model.InitRequest) (string, error) {
	var body strings.Builder
	fmt.Fprintf(&body, "# Gates: %s\n\n", req.Scope)
	for _, spec := range req.Gates {
		text, err := renderGateSpec(spec)
		if err != nil {
			return "", err
		}
		body.WriteString(text)
		body.WriteString("\n")
	}
	return body.String(), nil
}
func ValidateCreate(file string, exists bool) error {
	if exists {
		return fmt.Errorf("gate file already exists: %s", file)
	}
	return nil
}
func CreatedLedger(result model.InitResult, body string) model.InitResult {
	result.OK = true
	result.Created = true
	result.GateCount = len(Parse(body).Gates)
	return result
}
func PrepareAbandon(req model.AbandonRequest) (model.AbandonRequest, model.AbandonResult, error) {
	result := model.AbandonResult{SchemaVersion: model.SchemaVersion, File: req.File, GateID: req.GateID}
	if strings.TrimSpace(req.File) == "" {
		return req, result, fmt.Errorf("--file is required")
	}
	if strings.TrimSpace(req.GateID) == "" {
		return req, result, fmt.Errorf("--gate is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return req, result, fmt.Errorf("--reason is required")
	}
	return req, result, nil
}
func AbandonLedger(ledger *Ledger, req model.AbandonRequest) error {
	found := false
	for _, gate := range ledger.Gates {
		if gate.ID == req.GateID {
			if gate.Abandoned {
				return fmt.Errorf("gate %s is already abandoned", req.GateID)
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("gate %s not found in %s", req.GateID, req.File)
	}
	AppendAbandon(ledger, req.GateID, req.Reason)
	return nil
}
func RecordedAbandon(result model.AbandonResult) model.AbandonResult {
	result.OK = true
	result.Recorded = true
	return result
}

func gateFileSlug(scope string) string {
	var slug strings.Builder
	pendingDash := false
	runesWritten := 0
	for _, char := range strings.ToLower(strings.TrimSpace(scope)) {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			if slug.Len() > 0 {
				pendingDash = true
			}
			continue
		}
		if pendingDash {
			slug.WriteByte('-')
			pendingDash = false
		}
		slug.WriteRune(char)
		runesWritten++
		if runesWritten == 64 {
			break
		}
	}
	return strings.TrimSuffix(slug.String(), "-")
}

func renderGateSpec(spec string) (string, error) {
	parts := specSegments(spec)
	heading := strings.TrimSpace(parts[0])
	if heading == "" {
		return "", fmt.Errorf("gate spec %q has no title", spec)
	}
	// Probe: `gates init --gate nonsense` created a gate with no id (it
	// became "line3", which no plan or abandon refers to).
	if token, _, ok := strings.Cut(heading, ":"); !ok || token == "" || strings.ContainsAny(token, " \t") {
		return "", fmt.Errorf("gate spec %q: start with an id, e.g. \"G1: outcome | CHECK: command | EXPECT: text\"", spec)
	}
	lines := []string{"- [ ] " + heading}
	sawEvidence := false
	for _, raw := range parts[1:] {
		segment := strings.TrimSpace(raw)
		upper := strings.ToUpper(segment)
		switch {
		case strings.HasPrefix(upper, "CHECK:"):
			check := strings.TrimSpace(segment[len("CHECK:"):])
			if shelltoken.HasUnquotedControlOperator(check) {
				return "", fmt.Errorf("gate spec %q: CHECK must be one argv command; shell syntax is not executed (&&, ;) — put the sequence in a script file and run that file directly, or use python3 -c (sh -c and bash -c are refused)", spec)
			}
			if check == "" {
				return "", fmt.Errorf("gate spec %q: CHECK is empty; give the command that observes the outcome", spec)
			}
			lines = append(lines, "  CHECK: "+check)
		case strings.HasPrefix(upper, "EXPECT:"):
			// An empty EXPECT (exit code only) must not leave a trailing
			// space: QA's `git diff --check` failed on the committed ledger.
			lines = append(lines, strings.TrimRight("  EXPECT: "+strings.TrimSpace(segment[len("EXPECT:"):]), " "))
		case strings.HasPrefix(upper, "EVIDENCE:"):
			lines = append(lines, "  EVIDENCE: "+strings.TrimSpace(segment[len("EVIDENCE:"):]))
			sawEvidence = true
		default:
			return "", fmt.Errorf("gate spec %q has unknown segment %q (want CHECK:, EXPECT:, or EVIDENCE:)", spec, segment)
		}
	}
	if !sawEvidence {
		lines = append(lines, "  EVIDENCE: pending")
	}
	return strings.Join(lines, "\n"), nil
}

// specSegments splits a gate spec on |, then rejoins the pieces that follow
// EXPECT: up to the next keyword segment, so a regex alternation such as
// /^(a|b)$/m survives. Deno QA: EXPECT /ok|passed/ failed with "unknown
// segment". CHECK keeps the plain split: a | there is a shell pipe, which
// CHECK refuses anyway.
func specSegments(spec string) []string {
	raw := strings.Split(spec, "|")
	parts := []string{raw[0]}
	inExpect := false
	for _, piece := range raw[1:] {
		upper := strings.ToUpper(strings.TrimSpace(piece))
		keyword := strings.HasPrefix(upper, "CHECK:") || strings.HasPrefix(upper, "EXPECT:") || strings.HasPrefix(upper, "EVIDENCE:")
		if inExpect && !keyword {
			parts[len(parts)-1] += "|" + piece
			continue
		}
		inExpect = strings.HasPrefix(upper, "EXPECT:")
		parts = append(parts, piece)
	}
	return parts
}
