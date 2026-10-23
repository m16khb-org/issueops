package issueops

import (
	"encoding/json"
	"fmt"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// cycleOutcomeBucket keeps one metrics summary per deleted cycle record. Cleanup
// writes it in the same transaction that deletes the record, so the summary
// exists exactly when the record is gone.
var cycleOutcomeBucket = fmt.Sprintf("issueops_cycle_outcome_v%d", model.IssueOpsCycleOutcomeSchemaVersion)

func cycleOutcomeMutation(recordID string, outcome model.IssueOpsCycleOutcome) (port.RecordMutation, error) {
	if outcome.SchemaVersion != model.IssueOpsCycleOutcomeSchemaVersion || outcome.ID != recordID {
		return port.RecordMutation{}, fmt.Errorf("cycle outcome for %q does not match record %q at schema %d", outcome.ID, recordID, model.IssueOpsCycleOutcomeSchemaVersion)
	}
	data, err := json.Marshal(outcome)
	if err != nil {
		return port.RecordMutation{}, fmt.Errorf("encode cycle outcome %s: %w", recordID, err)
	}
	return port.RecordMutation{Bucket: cycleOutcomeBucket, ID: recordID, Data: data}, nil
}

// ListCycleOutcomes reads every retained cycle outcome. Rows that fail to decode
// or carry another schema are returned as unreadable IDs instead of guessed.
func ListCycleOutcomes(stateRoot string) ([]model.IssueOpsCycleOutcome, []string, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, nil, err
	}
	rows, err := db.GetAll(cycleOutcomeBucket)
	if err != nil {
		return nil, nil, err
	}
	outcomes := make([]model.IssueOpsCycleOutcome, 0, len(rows))
	var unreadable []string
	for _, row := range rows {
		var outcome model.IssueOpsCycleOutcome
		if err := json.Unmarshal(row.Data, &outcome); err != nil || outcome.SchemaVersion != model.IssueOpsCycleOutcomeSchemaVersion || outcome.ID != row.ID {
			unreadable = append(unreadable, row.ID)
			continue
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, unreadable, nil
}
