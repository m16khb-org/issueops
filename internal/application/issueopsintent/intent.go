package issueopsintent

import (
	"fmt"

	model "issueops/internal/contract/issueops"
	intentdomain "issueops/internal/domain/issueopsintent"
	"issueops/internal/domain/secretdetection"
	intentport "issueops/internal/port/issueopsintent"
)

func RecordIntent(store intentport.Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	intent, err := intentdomain.PrepareIntent(intentdomain.IntentDraft{RawRequest: req.RawRequest, InterpretedIntent: req.InterpretedIntent, SuccessCriteria: req.SuccessCriteria, Constraints: req.Constraints, Ambiguities: req.Ambiguities, NonGoals: req.NonGoals})
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	intentClass, err := model.NormalizeIntentClass(req.IntentClass)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.Intent = &model.IssueOpsIntentContract{RawRequest: intent.RawRequest, InterpretedIntent: intent.InterpretedIntent, SuccessCriteria: intent.SuccessCriteria, Constraints: intent.Constraints, Ambiguities: intent.Ambiguities, NonGoals: intent.NonGoals, IntentClass: intentClass, RecordedAt: store.Now()}
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
