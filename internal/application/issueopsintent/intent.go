package issueopsintent

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	intentdomain "issueops/internal/domain/issueopsintent"
	"issueops/internal/domain/policy"
	"issueops/internal/domain/secretdetection"
	intentport "issueops/internal/port/issueopsintent"
)

func RecordIntent(store intentport.Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	rawRequest := strings.TrimSpace(req.RawRequest)
	if rawRequest == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("raw_request is required")
	}
	interpretedIntent := strings.TrimSpace(req.InterpretedIntent)
	if interpretedIntent == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent is required")
	}
	if interpretedIntent == rawRequest {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent must differ from raw_request")
	}
	if !intentdomain.MateriallyDifferentIntent(rawRequest, interpretedIntent) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent must materially differ from raw_request")
	}
	successCriteria := intentdomain.CleanTextValues(req.SuccessCriteria)
	if len(successCriteria) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("success_criteria is required")
	}
	intentClass, err := model.NormalizeIntentClass(req.IntentClass)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.Intent = &model.IssueOpsIntentContract{
		RawRequest:        policy.RedactFreeform(rawRequest),
		InterpretedIntent: policy.RedactFreeform(interpretedIntent),
		SuccessCriteria:   successCriteria,
		Constraints:       intentdomain.CleanTextValues(req.Constraints),
		Ambiguities:       intentdomain.CleanTextValues(req.Ambiguities),
		NonGoals:          intentdomain.CleanTextValues(req.NonGoals),
		IntentClass:       intentClass,
		RecordedAt:        store.Now(),
	}
	if secretdetection.Contains(intentdomain.Render(IntentDocument(record))) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("intent contains secret-like values; redact them before recording")
	}
	return store.TouchWrite(stateRoot, record)
}

// IntentDocument omits RecordedAt so rerecording does not alter sealed artifact bytes.
func IntentDocument(record model.IssueOpsRecord) intentdomain.Document {
	doc := intentdomain.Document{LifecycleID: record.ID, IssueURL: record.IssueURL}
	if record.Intent == nil {
		return doc
	}
	doc.IntentClass = record.Intent.IntentClass
	doc.RawRequest = record.Intent.RawRequest
	doc.InterpretedIntent = record.Intent.InterpretedIntent
	doc.SuccessCriteria = record.Intent.SuccessCriteria
	doc.NonGoals = record.Intent.NonGoals
	doc.Constraints = record.Intent.Constraints
	doc.Ambiguities = record.Intent.Ambiguities
	return doc
}
