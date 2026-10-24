package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ValidateRemoteCreateInputs(kind, title, body string, labels, assignees []string) error {
	values := []struct {
		field string
		value string
	}{
		{field: "title", value: title},
		{field: "body", value: body},
	}
	for _, label := range labels {
		values = append(values, struct {
			field string
			value string
		}{field: "label", value: label})
	}
	for _, assignee := range assignees {
		values = append(values, struct {
			field string
			value string
		}{field: "assignee", value: assignee})
	}
	for _, candidate := range values {
		if RedactFreeform(candidate.value) != candidate.value {
			return fmt.Errorf("%s %s contains secret-like content%s", kind, candidate.field, secretLikeLocation(candidate.value))
		}
	}
	return nil
}

// secretLikeLocation names the line and the rule SecretLikeArg matched,
// never the value. QA agents rewrote whole bodies by guesswork when the
// refusal named only the field. Detection itself is SecretLikeArg, unchanged.
func secretLikeLocation(value string) string {
	offset, rule := -1, ""
	for _, candidate := range []struct {
		loc  []int
		rule string
	}{
		{secretArgRe().FindStringIndex(value), "an assignment such as token=, password=, api_key= or an Authorization: Bearer header"},
		{knownTokenRe().FindStringIndex(value), "a provider token format (ghp_, github_pat_, xox*, sk-, AKIA)"},
		{secretPathRe().FindStringIndex(filepath.ToSlash(value)), "a credential-like path or word (.env, id_rsa, *.pem, *.key, credentials, secret) at the start or after a /"},
	} {
		if candidate.loc != nil && (offset < 0 || candidate.loc[0] < offset) {
			offset, rule = candidate.loc[0], candidate.rule
		}
	}
	if offset < 0 {
		return ""
	}
	return fmt.Sprintf(" (line %d: %s; rewrite that line without it)", strings.Count(value[:offset], "\n")+1, rule)
}
