package issueopsreview

// ReviewGateEvidence is the portion of a persisted review relevant to its
// publication gate. The record adapter supplies presence separately.
type ReviewGateEvidence struct {
	Present             bool
	Verdict             string
	ReviewedFingerprint string
}

// SchemaGateEvidence 타입은 저장된 스키마 검토 중 게시 게이트가 필요로 하는
// 부분이다. 면제 여부와 사유, 측정과 근거 수, 검토한 지문을 담는다.
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
