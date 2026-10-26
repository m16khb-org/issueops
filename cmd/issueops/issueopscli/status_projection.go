package issueopscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"issueops/cmd/issueops/cliflag"
	issueopscontract "issueops/internal/contract/issueops"
)

// statusProjection keeps ok, id, phase and the requested top-level fields of a
// status record. QA transcripts: skills read one field of `status --json`
// (branch_prepare, remote_artifact, a review) and agents received the whole
// record, about 12 KB at the done phase. A requested field the record has not
// recorded yet is listed under "absent"; an unknown name is refused with the
// valid names, so a typo is not mistaken for an absent value.
func statusProjection(record issueopscontract.IssueOpsRecord, fields []string) (map[string]any, error) {
	known := recordFieldNames()
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	var full map[string]json.RawMessage
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, err
	}
	out := map[string]any{"ok": record.OK, "id": record.ID, "phase": record.Phase}
	absent := []string{}
	for _, name := range fields {
		for part := range strings.SplitSeq(name, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if !slices.Contains(known, part) {
				message := fmt.Sprintf("unknown status field %q (top-level record fields only)", part)
				if suggestions := cliflag.Suggest(part, known, 3); len(suggestions) > 0 {
					message += "; did you mean: " + strings.Join(suggestions, ", ")
				}
				return nil, errors.New(message + "; omit --field for the whole record")
			}
			if value, ok := full[part]; ok {
				out[part] = value
			} else if !slices.Contains(absent, part) {
				absent = append(absent, part)
			}
		}
	}
	if len(absent) > 0 {
		out["absent"] = absent
	}
	return out, nil
}

func recordFieldNames() []string {
	recordType := reflect.TypeFor[issueopscontract.IssueOpsRecord]()
	names := make([]string, 0, recordType.NumField())
	for field := range recordType.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names = append(names, name)
		}
	}
	return names
}
