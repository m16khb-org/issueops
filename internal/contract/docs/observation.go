package docs

// Candidate preserves the literal filesystem path alongside its root-relative
// identity. RelPath is empty when the observer cannot derive that identity.
type Candidate struct {
	Path    string
	RelPath string
}
