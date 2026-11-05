// Package judgement holds the domain rules for host-agent judgement prompts
// and for strictly decoding their structured JSON results.
package judgement

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"issueops/internal/domain/policy"
	"issueops/internal/domain/prompt"
)

// BuildJSONSchemaSection renders the prompt section instructing the host agent
// to answer with exactly one JSON object. Non-empty fieldTypes become schema
// bullet lines and a non-empty example is appended after them.
func BuildJSONSchemaSection(example string, fieldTypes []string) prompt.DataSection {
	lines := []string{
		"Return exactly one JSON object matching this response schema.",
		"Do not include prose before or after the JSON object.",
		"Do not include Markdown unless the caller explicitly permits a fenced json block.",
	}
	for _, fieldType := range fieldTypes {
		fieldType = strings.TrimSpace(fieldType)
		if fieldType != "" {
			lines = append(lines, "- "+fieldType)
		}
	}
	example = strings.TrimSpace(example)
	if example != "" {
		lines = append(lines, "", "Example:", example)
	}
	return prompt.DataSection{
		Title:   "Host-Agent Judgement Response Schema",
		Content: strings.Join(lines, "\n"),
	}
}

// DecodeStructuredJSONObject is a pure decoder for host-agent-authored result
// files. It has no invoke handle, so it deliberately does not reprompt or retry
// on malformed output; the caller must provide a corrected result file.
func DecodeStructuredJSONObject(label string, out []byte, target any) error {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "host-agent judgement"
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return fmt.Errorf("%s output is empty", label)
	}
	err := decodeStrictJSONObject(trimmed, target)
	if err == nil {
		return nil
	}
	fenced, ok, fenceErr := extractFencedJSON(trimmed)
	if fenceErr != nil {
		return fmt.Errorf("%s output is not strict JSON: %w", label, fenceErr)
	}
	if ok {
		if err := decodeStrictJSONObject(fenced, target); err != nil {
			return fmt.Errorf("%s fenced JSON is invalid: %w", label, err)
		}
		return nil
	}
	return fmt.Errorf("%s output is not strict JSON: %w; output=%s", label, err, boundedOutputText(string(trimmed)))
}

func decodeStrictJSONObject(out []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("multiple JSON values")
	}
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return fmt.Errorf("multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func extractFencedJSON(out []byte) ([]byte, bool, error) {
	text := string(out)
	offset := 0
	for {
		startRel := strings.Index(text[offset:], "```json")
		if startRel < 0 {
			return nil, false, nil
		}
		start := offset + startRel
		langEnd := start + len("```json")
		if langEnd < len(text) {
			next, _ := utf8RuneAt(text, langEnd)
			if next != '\n' && next != '\r' && next != ' ' && next != '\t' {
				offset = langEnd
				continue
			}
		}
		contentStart := strings.IndexAny(text[langEnd:], "\n\r")
		if contentStart < 0 {
			return nil, true, fmt.Errorf("json fence has no content line")
		}
		contentStart += langEnd + 1
		end := strings.Index(text[contentStart:], "```")
		if end < 0 {
			return nil, true, fmt.Errorf("json fence is not closed")
		}
		return []byte(strings.TrimSpace(text[contentStart : contentStart+end])), true, nil
	}
}

func boundedOutputText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1000 {
		return policy.TruncateBytes(s, 1000) + "...[truncated]"
	}
	return s
}

func utf8RuneAt(s string, i int) (rune, int) {
	if i >= len(s) {
		return 0, 0
	}
	return utf8.DecodeRuneInString(s[i:])
}
