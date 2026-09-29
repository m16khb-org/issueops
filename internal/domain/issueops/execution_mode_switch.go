package issueops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	model "issueops/internal/contract/issueops"
)

func ResetExecutionForModeSwitch(record model.IssueOpsRecord) model.IssueOpsRecord {
	record.Execution = nil
	record.WorktreePath = ""
	record.PlanPath = ""
	return record
}

func ModeSwitchRecordFingerprint(record model.IssueOpsRecord) (string, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
