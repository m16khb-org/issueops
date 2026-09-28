package issueops

import (
	"encoding/json"
	"fmt"

	"issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

func publicationActor(actor contract.Actor) issueops.NativeActor {
	result := issueops.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &issueops.NativeProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	if actor.ProcessAncestry != nil {
		result.ProcessAncestry = make([]issueops.NativeProcessReceipt, len(actor.ProcessAncestry))
		for index, receipt := range actor.ProcessAncestry {
			result.ProcessAncestry[index] = issueops.NativeProcessReceipt{
				PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
			}
		}
	}
	return result
}

func publicationIntentSnapshot(intent contract.Intent) (issueops.IssueOpsRecord, contract.IntentPayload, error) {
	var record issueops.IssueOpsRecord
	if len(intent.Record.Raw) == 0 {
		return record, contract.IntentPayload{}, fmt.Errorf("publication record raw bytes are required")
	}
	if err := json.Unmarshal(intent.Record.Raw, &record); err != nil {
		return record, contract.IntentPayload{}, fmt.Errorf("decode publication record: %w", err)
	}
	if len(intent.Raw) == 0 {
		return record, contract.IntentPayload{}, fmt.Errorf("remote publication intent raw bytes are required")
	}
	var payload contract.IntentPayload
	if err := json.Unmarshal(intent.Raw, &payload); err != nil {
		return record, payload, fmt.Errorf("decode remote publication intent: %w", err)
	}
	if payload.SchemaVersion != issueops.IssueOpsSchemaVersion || payload.OperationID == "" || payload.OperationID != intent.OperationID || payload.Generation == 0 {
		return record, payload, fmt.Errorf("remote publication intent state is invalid")
	}
	return record, payload, nil
}
