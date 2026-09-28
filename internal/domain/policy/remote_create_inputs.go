package policy

import "fmt"

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
			return fmt.Errorf("%s %s contains secret-like content", kind, candidate.field)
		}
	}
	return nil
}
