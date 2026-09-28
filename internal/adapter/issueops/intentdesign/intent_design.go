package intentdesign

import (
	"errors"
	"fmt"
	issueopscontract "issueops/internal/contract/issueops"
	"strings"
	"time"

	reviewapp "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/issueopsintent"
	reviewdomain "issueops/internal/domain/issueopsreview"
	"issueops/internal/domain/policy"
	"issueops/internal/domain/secretdetection"
	reviewport "issueops/internal/port/issueopsreview"
)

type Store struct {
	Read          func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite    func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanReadiness func(record model.IssueOpsRecord) model.IssueOpsReadiness
}

const (
	DesignReviewEvidenceExample  = issueopscontract.IssueOpsDesignReviewEvidenceExample
	designReviewEvidenceGuidance = `approved design review requires design_review_evidence: this is not a separate flag or decision record; add --verification "design review checked alternatives and risks" or a Korean equivalent such as "설계 검토 완료: 대안과 위험 확인"`
)

func RecordIntent(store Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
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
	if !issueopsintent.MateriallyDifferentIntent(rawRequest, interpretedIntent) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent must materially differ from raw_request")
	}
	successCriteria := issueopsintent.CleanTextValues(req.SuccessCriteria)
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
		Constraints:       issueopsintent.CleanTextValues(req.Constraints),
		Ambiguities:       issueopsintent.CleanTextValues(req.Ambiguities),
		NonGoals:          issueopsintent.CleanTextValues(req.NonGoals),
		IntentClass:       intentClass,
		RecordedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	// redaction은 키워드 뒤에 등호가 오는 형태만 지우므로, 봉인 artifact가 거부할
	// 콜론 형태는 여기서 미리 막아 실패 지점을 기록 시점으로 당긴다.
	if secretdetection.Contains(issueopsintent.Render(IntentDocument(record))) {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("intent contains secret-like values; redact them before recording")
	}
	return store.TouchWrite(stateRoot, record)
}

func RecordDesignReview(store Store, stateRoot, id string, req model.IssueOpsDesignReviewRequest) (model.IssueOpsRecord, error) {
	record, err := reviewapp.RecordDesignReview(reviewport.DesignReviewStore{
		Read:          store.Read,
		PlanReadiness: store.PlanReadiness,
		TouchWrite:    store.TouchWrite,
		Now:           func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}, stateRoot, id, req)
	if errors.Is(err, reviewdomain.ErrMissingDesignReviewEvidence) {
		return model.IssueOpsRecord{OK: false}, errors.New(designReviewEvidenceGuidance)
	}
	return record, err
}

// IntentDocument는 record.intent를 봉인 intent artifact의 렌더 입력으로 옮긴다.
// RecordedAt은 의도적으로 옮기지 않는다: 재기록마다 바뀌어 봉인 바이트를 흔든다.
func IntentDocument(record model.IssueOpsRecord) issueopsintent.Document {
	doc := issueopsintent.Document{LifecycleID: record.ID, IssueURL: record.IssueURL}
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
