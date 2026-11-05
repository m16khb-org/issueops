// Package issueopspublication defines the contract types shared by the publication use cases and adapters:
// create commands, provider requests and results, candidate inventories, and persisted create intents.
// It holds data and defensive-copy helpers only; decision rules live in internal/domain/issueopspublication.
package issueopspublication

// Issue-create intent statuses persisted for a create operation, from pending through completed or a failure state.
const (
	IssueCreateIntentPending            = "pending"
	IssueCreateIntentNotInvoked         = "not_invoked"
	IssueCreateIntentInvokedUnknown     = "invoked_unknown"
	IssueCreateIntentURLObserved        = "url_observed"
	IssueCreateIntentVerificationFailed = "verification_failed"
	IssueCreateIntentReceiptFailed      = "receipt_failed"
	IssueCreateIntentCompleted          = "completed"
)

// InvocationState describes what is known about whether the provider create call was invoked.
type InvocationState string

// Invocation states: InvocationUnknown means the call may have happened; InvocationNotInvokedProven means it provably did not.
const (
	InvocationUnknown          InvocationState = "unknown"
	InvocationNotInvokedProven InvocationState = "not_invoked_proven"
)

// ProcessReceipt identifies an OS process by PID, start time, and executable path so process identity survives PID reuse.
type ProcessReceipt struct {
	PID        int
	StartedAt  string
	Executable string
}

// Actor identifies the host session and agent that issues a create command, with the session process and its ancestry.
type Actor struct {
	Host            string
	SessionID       string
	AgentID         string
	SessionProcess  *ProcessReceipt
	ProcessAncestry []ProcessReceipt
}

// Clone returns a deep copy of a, duplicating the process pointer and ancestry slice.
func (a Actor) Clone() Actor {
	cloned := a
	if a.SessionProcess != nil {
		process := *a.SessionProcess
		cloned.SessionProcess = &process
	}
	if a.ProcessAncestry != nil {
		cloned.ProcessAncestry = append([]ProcessReceipt{}, a.ProcessAncestry...)
	}
	return cloned
}

// CreateCommand is the caller's request to create a PR/MR: content fields, labels and assignees,
// the expected execution generation, the acting Actor, and whether Confirm authorizes the remote write.
type CreateCommand struct {
	ID                 string
	Provider           string
	Title              string
	Body               string
	Head               string
	Base               string
	Labels             []string
	Assignees          []string
	ExpectedGeneration uint64
	Actor              Actor
	CWD                string
	Confirm            bool
}

// Clone returns a copy of c whose label, assignee, and actor slices do not alias c.
func (c CreateCommand) Clone() CreateCommand {
	cloned := c
	cloned.Labels = cloneStrings(c.Labels)
	cloned.Assignees = cloneStrings(c.Assignees)
	cloned.Actor = c.Actor.Clone()
	return cloned
}

// ProviderCreateRequest is the serialized request handed to the provider to create a PR/MR.
// It is stored in the create intent, so its JSON tags are part of the persisted format.
type ProviderCreateRequest struct {
	Repo            string   `json:"repo"`
	ProjectKey      string   `json:"project_key,omitempty"`
	Title           string   `json:"title"`
	Body            string   `json:"body"`
	HeadBranch      string   `json:"head_branch"`
	BaseBranch      string   `json:"base_branch"`
	Labels          []string `json:"labels"`
	Assignees       []string `json:"assignees"`
	Draft           bool     `json:"draft"`
	ExpectedHeadSHA string   `json:"expected_head_sha,omitempty"`
	Confirm         bool     `json:"confirm"`
	Host            string   `json:"host,omitempty"`
	SessionID       string   `json:"session_id,omitempty"`
	AgentID         string   `json:"agent_id,omitempty"`
	CWD             string   `json:"cwd,omitempty"`
}

// Clone returns a copy of r whose label and assignee slices do not alias r.
func (r ProviderCreateRequest) Clone() ProviderCreateRequest {
	cloned := r
	cloned.Labels = cloneStrings(r.Labels)
	cloned.Assignees = cloneStrings(r.Assignees)
	return cloned
}

// ProviderCreateResult is the provider's reply to a create call: success flag, resulting URL and number, and an optional preview.
type ProviderCreateResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`
	Number  string `json:"number"`
	Preview string `json:"preview,omitempty"`
}

// CreateEligibility holds the facts checked before a create may proceed:
// provider and kind, confirmation, PR phase, active execution, absence of pending intents or artifacts, and branch and label authority.
type CreateEligibility struct {
	Provider                 string
	Kind                     string
	Confirm                  bool
	PhasePR                  bool
	ExecutionActive          bool
	NoPending                bool
	NoArtifact               bool
	BranchAuthority          bool
	CanonicalLabelsAssignees bool
}

// PreparedCreate bundles a validated command with its provider request and the eligibility facts it was checked against.
type PreparedCreate struct {
	Command     CreateCommand
	Request     ProviderCreateRequest
	Eligibility CreateEligibility
}

// Candidate is an existing remote PR/MR considered for adoption during reconcile.
// BodySHA256 is the digest of the body rather than the body itself.
type Candidate struct {
	URL              string
	ProjectKey       string
	SourceProjectKey string
	HeadBranch       string
	BaseBranch       string
	HeadSHA          string
	Title            string
	BodySHA256       string
	Labels           []string
	Assignees        []string
	Draft            bool
	State            string
}

// Clone returns a copy of c whose label and assignee slices do not alias c.
func (c Candidate) Clone() Candidate {
	cloned := c
	cloned.Labels = cloneStrings(c.Labels)
	cloned.Assignees = cloneStrings(c.Assignees)
	return cloned
}

// Inventory is the set of remote candidates found for an operation.
// AuthoritativeZero is true when an empty Candidates list is a verified absence rather than an incomplete search.
type Inventory struct {
	Candidates        []Candidate
	AuthoritativeZero bool
}

// Clone returns a deep copy of i, cloning every candidate.
func (i Inventory) Clone() Inventory {
	cloned := i
	if i.Candidates != nil {
		cloned.Candidates = make([]Candidate, len(i.Candidates))
		for index, candidate := range i.Candidates {
			cloned.Candidates[index] = candidate.Clone()
		}
	}
	return cloned
}

// RecordSnapshot is a journal record's ID and raw bytes as read, used to detect concurrent change.
type RecordSnapshot struct {
	ID  string
	Raw []byte
}

// Clone returns a copy of r with its own Raw byte slice.
func (r RecordSnapshot) Clone() RecordSnapshot {
	cloned := r
	cloned.Raw = cloneBytes(r.Raw)
	return cloned
}

// Intent is a decoded create intent: the journal record it came from, the operation identity,
// the provider request, the invocation state with retry count and known URL, and the original raw payload.
type Intent struct {
	Record          RecordSnapshot
	OperationID     string
	Generation      uint64
	Provider        string
	Kind            string
	Request         ProviderCreateRequest
	InvocationState InvocationState
	RetryCount      int
	KnownURL        string
	Eligibility     CreateEligibility
	Raw             []byte
	// PendingStartedAt is when the pending create was recorded (RFC3339).
	PendingStartedAt string
}

// Clone returns a deep copy of i, including the record, request, and raw bytes.
func (i Intent) Clone() Intent {
	cloned := i
	cloned.Record = i.Record.Clone()
	cloned.Request = i.Request.Clone()
	cloned.Raw = cloneBytes(i.Raw)
	return cloned
}

// ReconcileResult reports the outcome of reconciling an intent: the resulting record snapshot,
// whether it was reconciled, a result Code, and whether the external provider state was inspected.
type ReconcileResult struct {
	Record                 RecordSnapshot
	Reconciled             bool
	Code                   string
	ExternalStateInspected bool
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}
