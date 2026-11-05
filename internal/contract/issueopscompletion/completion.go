// Package issueopscompletion defines the transport-neutral execution
// completion contract shared by domain, application, and adapters.
package issueopscompletion

import leasecontract "issueops/internal/contract/issueopslease"

// Execution aliases the lease contract's execution state under this package's name.
// application/issueopscompletion은 completion domain·contract만 import할 수 있어서
// lease 계약의 Execution과 ErrExecutionNotPrepared를 여기서 이 이름으로 둔다.
type Execution = leasecontract.Execution

// ErrExecutionNotPrepared is returned when completion targets a cycle whose execution was never prepared.
var ErrExecutionNotPrepared = leasecontract.ErrExecutionNotPrepared

// ProcessReceipt identifies one OS process by PID, start time, and executable so a later check can tell
// it apart from a reused PID.
type ProcessReceipt struct {
	PID        int
	StartedAt  string
	Executable string
}

// Actor identifies who acts on a cycle: the host, session, and agent IDs plus the verified process.
type Actor struct {
	Host      string
	SessionID string
	AgentID   string
	Process   *ProcessReceipt
}

// Lease is the execution lease as completion sees it: its generation, status, holder, claim token digest,
// and the timestamps and reason recorded when it was claimed, released, or replaced.
type Lease struct {
	Generation        uint64
	Status            string
	Holder            *Actor
	ClaimTokenSHA256  string
	ClaimedAt         string
	ReleasedAt        string
	ReplacedAt        string
	ReplacementReason string
}

// Completion is the evidence recorded when a cycle completes: lease generation, final head, verification
// report path and notes, remote artifact URL, and the completion time.
type Completion struct {
	Generation             uint64
	FinalHead              string
	VerificationReportPath string
	Verification           []string
	RemoteArtifactURL      string
	CompletedAt            string
}

// LedgerEntry is the phase ledger record for one phase: when it started and finished, and its artifacts,
// missing items, and notes.
type LedgerEntry struct {
	Phase       string
	EnteredAt   string
	CompletedAt string
	Artifacts   []string
	Missing     []string
	Notes       []string
}

// Command is the validated request to complete a cycle: the lease generation, the verified actor,
// and the final head, verification evidence, and remote artifact URL to record.
type Command struct {
	Generation             uint64
	Actor                  Actor
	FinalHead              string
	VerificationReportPath string
	Verification           []string
	RemoteArtifactURL      string
}

// RemoteArtifact describes the remote issue or pull request linked to a cycle: provider, kind, URL, labels,
// assignees, when it was last verified, and its target branch.
type RemoteArtifact struct {
	Provider     string
	Kind         string
	URL          string
	Labels       []string
	Assignees    []string
	VerifiedAt   string
	TargetBranch string
}

// RecordSnapshot is the minimum persisted projection required to decide and
// apply one completion transition. Persistence-specific raw sidecars stay in
// the outbound repository.
type RecordSnapshot struct {
	ID            string
	Prepared      bool
	Phase         string
	IssueURL      string
	CanonicalRoot string
	Lease         Lease
	Completion    *Completion
	Artifact      *RemoteArtifact
	BaseBranch    string
	// CodeProjectKey mirrors the cycle's sealed code project; empty means the
	// code lives in the issue's own project.
	CodeProjectKey string
	Ledger         map[string]LedgerEntry
	// PRHead is the canonical HEAD recorded when the cycle entered the pr phase; empty for
	// records that entered pr before it was recorded.
	PRHead string
}

// Clone returns a deep copy of the snapshot, so a transition can edit the result without changing the original.
func (r RecordSnapshot) Clone() RecordSnapshot {
	result := r
	if r.Lease.Holder != nil {
		holder := *r.Lease.Holder
		if holder.Process != nil {
			process := *holder.Process
			holder.Process = &process
		}
		result.Lease.Holder = &holder
	}
	if r.Completion != nil {
		completion := *r.Completion
		completion.Verification = append([]string(nil), completion.Verification...)
		result.Completion = &completion
	}
	if r.Artifact != nil {
		artifact := *r.Artifact
		artifact.Labels = append([]string(nil), artifact.Labels...)
		artifact.Assignees = append([]string(nil), artifact.Assignees...)
		result.Artifact = &artifact
	}
	result.Ledger = make(map[string]LedgerEntry, len(r.Ledger))
	for phase, entry := range r.Ledger {
		entry.Artifacts = append([]string(nil), entry.Artifacts...)
		entry.Missing = append([]string(nil), entry.Missing...)
		entry.Notes = append([]string(nil), entry.Notes...)
		result.Ledger[phase] = entry
	}
	return result
}
