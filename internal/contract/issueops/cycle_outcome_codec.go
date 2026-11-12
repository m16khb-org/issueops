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

// DecodeCycleOutcome 함수는 recordID 아래 저장된 보존 행 바이트를 읽는다. JSON이
// 깨졌거나, CycleOutcomeSchemaVersion이 아니거나, 다른 cycle의 outcome이면 오류를
// 돌려준다.
func DecodeCycleOutcome(recordID string, data []byte) (CycleOutcome, error) {
	var outcome CycleOutcome
	// 모르는 필드는 거부하지 않는다. 뒤 버전이 더한 필드가 있어도 행을 읽어야 한다.
	if err := json.Unmarshal(data, &outcome); err != nil {
		return CycleOutcome{}, fmt.Errorf("decode cycle outcome %s: %w", recordID, err)
	}
	if outcome.SchemaVersion != CycleOutcomeSchemaVersion {
		return CycleOutcome{}, fmt.Errorf("cycle outcome %s has schema %d, want %d", recordID, outcome.SchemaVersion, CycleOutcomeSchemaVersion)
	}
	if outcome.ID != recordID {
		return CycleOutcome{}, fmt.Errorf("cycle outcome row %s holds outcome for %q", recordID, outcome.ID)
	}
	return outcome, nil
}
