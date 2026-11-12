package issueops

import (
	"encoding/json"
	"fmt"
)

// EncodeCycleOutcome 함수는 recordID 아래 저장할 보존 행 바이트를 만든다. 현재
// CycleOutcomeSchemaVersion이 아니거나 다른 cycle의 outcome이면 오류를 돌려준다.
func EncodeCycleOutcome(recordID string, outcome CycleOutcome) ([]byte, error) {
	if outcome.SchemaVersion != CycleOutcomeSchemaVersion || outcome.ID != recordID {
		return nil, fmt.Errorf("cycle outcome for %q does not match record %q at schema %d", outcome.ID, recordID, CycleOutcomeSchemaVersion)
	}
	data, err := json.Marshal(outcome)
	if err != nil {
		return nil, fmt.Errorf("encode cycle outcome %s: %w", recordID, err)
	}
	return data, nil
}
