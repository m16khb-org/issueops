package issueopsreview

import (
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordSchemaEvidence stores the schema evidence of record id: measurements and
// sources, or an explicit waiver with a rationale. The evidence is bound to the
// local change fingerprint observed before the lock and is rejected when the
// change set moved by commit time.
func RecordSchemaEvidence(store reviewport.EvidenceReviewStore, stateRoot, id string, req model.SchemaEvidenceRequest) (model.Record, error) {
	measurements := cleanReviewValues(req.Measurements)
	sources := cleanReviewValues(req.Sources)
	rationale := strings.TrimSpace(req.WaiverRationale)
	if err := reviewdomain.ValidateSchemaEvidenceRecord(req.Waive, rationale, len(measurements), len(sources)); err != nil {
		return model.Record{OK: false}, err
	}
	observed, err := store.Read(stateRoot, id)
	if err != nil {
		return model.Record{OK: false}, err
	}
	fingerprint := store.Fingerprint(observed)
	var result model.Record
	err = store.WithLock(stateRoot, id, func() error {
		current, err := store.Read(stateRoot, id)
		if err != nil {
			return err
		}
		if err := store.ValidateMutation(current); err != nil {
			return err
		}
		if err := issueopsdomain.ValidateEvidenceRecordingPhase(current.Phase, "schema evidence"); err != nil {
			return err
		}
		if err := validateCurrentChangeObservation(observed, current); err != nil {
			return err
		}
		now := store.Now()
		current.SchemaEvidence = &model.SchemaEvidence{
			Measurements: measurements, Sources: sources,
			Waived: req.Waive, WaiverRationale: strings.Join(cleanReviewValues([]string{rationale}), ""),
			ReviewedFingerprint: fingerprint, RecordedAt: now,
		}
		current.UpdatedAt = now
		result, err = store.Write(stateRoot, current)
		return err
	})
	if err != nil {
		return model.Record{OK: false}, err
	}
	return result, nil
}
