package devilsadvocate

import (
	"errors"
	"fmt"
	"time"

	reviewapp "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// Store is the persistence seam, mirroring the compatibility-review recorder.
// PlanDigest resolves the sha256 of the plan the cycle will implement (the
// linked plan file, or the staged plan artifact when no file is linked) so the
// verdict is bound to the plan content it reviewed. It fails when the cycle has
// no plan yet — a devil's advocate without a plan has nothing to review.
type Store = reviewport.DevilsAdvocateStore

// Record validates the request and persists it as the record's
// DevilsAdvocateReview, keeping the earlier rounds of this plan phase in
// History. It does not gate on readiness: the devil's advocate runs on the
// completed plan, and the fail-closed gate lives at implement entry.
func Record(store Store, stateRoot, id string, req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	record, err := reviewapp.RecordDevilsAdvocate(store, stateRoot, id, req, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		if capErr, ok := errors.AsType[*reviewdomain.ReviseRoundCapError](err); ok {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf(
				"revise round cap reached: cycle %s already recorded %d unwaived revise verdicts on this plan phase, so the plan is not converging; "+
					"record a stop verdict, reflect it with issueops remote reflect-devils-advocate --id %s --confirm, then issueops regress --id %s --reason <TEXT>; "+
					"or record this round with --waive --waiver-rationale <TEXT>",
				id, capErr.Count, id, id)
		}
		return record, err
	}
	return record, nil
}

// Validate normalizes and checks a devil's-advocate request:
//   - verdict must be pass | revise | stop
//   - reviewer_context must be subagent | inline (audit field, always recorded)
//   - a pass needs at least one finding (what was attacked and why it failed)
//   - a stop/revise verdict needs concrete findings OR an explicit waiver
//   - a waiver needs a rationale
func Validate(req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsDevilsAdvocateReview, error) {
	return reviewdomain.ValidateReview(req, time.Now().UTC().Format(time.RFC3339Nano))
}
