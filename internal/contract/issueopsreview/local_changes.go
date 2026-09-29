package issueopsreview

// LocalChangeObservation binds readiness and review classification to the same verified snapshot.
type LocalChangeObservation struct {
	Paths       []string
	Fingerprint string
	Verified    bool
}

type ChangeBaseCandidate struct {
	Ref           string
	LiteralObject bool
}

type UpstreamFetch struct {
	Root   string
	Failed bool
	Stderr string
}
