package issueopsreview

// ReviewGateEvidence is the portion of a persisted review relevant to its
// publication gate. The record adapter supplies presence separately.
type ReviewGateEvidence struct {
	Present             bool
	Verdict             string
	ReviewedFingerprint string
}

type SchemaGateEvidence struct {
	Present             bool
	Waived              bool
	WaiverRationale     string
	MeasurementCount    int
	SourceCount         int
	ReviewedFingerprint string
}

// ChangeObservationIdentity contains the record inputs used for local change
// observation. A branch preparation with empty fields is distinct from none.
type ChangeObservationIdentity struct {
	Root            string
	HasPreparedBase bool
	BaseSHA         string
	BaseBranch      string
}
