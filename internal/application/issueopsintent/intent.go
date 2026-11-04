// Package issueopsintent is the application service that records the intent
// contract and plan preparation of an IssueOps lifecycle record.
package issueopsintent

import (
	"fmt"

	model "issueops/internal/contract/issueops"
	intentdomain "issueops/internal/domain/issueopsintent"
	"issueops/internal/domain/secretdetection"
	intentport "issueops/internal/port/issueopsintent"
)

// RecordIntent validates req, stores it as the intent contract of record id and
// returns the updated record. It rejects secret-like values anywhere in the
// rendered intent document and invalid intent classes.
func RecordIntent(store intentport.Store, stateRoot, id string, req model.IntentRecordRequest) (model.Record, error) {
	intent, err := intentdomain.PrepareIntent(intentdomain.IntentDraft{RawRequest: req.RawRequest, InterpretedIntent: req.InterpretedIntent, SuccessCriteria: req.SuccessCriteria, Constraints: req.Constraints, Ambiguities: req.Ambiguities, NonGoals: req.NonGoals})
	if err != nil {
		return model.Record{OK: false}, err
	}
	intentClass, err := model.NormalizeIntentClass(req.IntentClass)
	if err != nil {
		return model.Record{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.Intent = &model.IntentContract{RawRequest: intent.RawRequest, InterpretedIntent: intent.InterpretedIntent, SuccessCriteria: intent.SuccessCriteria, Constraints: intent.Constraints, Ambiguities: intent.Ambiguities, NonGoals: intent.NonGoals, IntentClass: intentClass, RecordedAt: store.Now()}
	if match, found := secretdetection.Find(intentdomain.Render(IntentDocument(record))); found {
		return model.Record{OK: false}, fmt.Errorf("intent contains a secret-like value (%s); redact it before recording", match.Kind)
	}
	return store.TouchWrite(stateRoot, record)
}

// IntentDocument omits RecordedAt so rerecording does not alter sealed artifact bytes.
func IntentDocument(record model.Record) intentdomain.Document {
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
