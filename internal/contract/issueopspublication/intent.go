package issueopspublication

// RemoteIntentKind is the external-intent kind recorded in the publication journal for a pending remote PR/MR creation.
const RemoteIntentKind = "remote_pr_create"

// IntentPayload is the JSON payload persisted with a remote create intent so a later reconcile can replay or verify the provider call.
// Its JSON tags are part of the persisted format.
type IntentPayload struct {
	SchemaVersion   int                   `json:"schema_version"`
	OperationID     string                `json:"operation_id"`
	Generation      uint64                `json:"generation"`
	Provider        string                `json:"provider"`
	Kind            string                `json:"kind"`
	Request         ProviderCreateRequest `json:"request"`
	InvocationState string                `json:"invocation_state"`
	RetryCount      int                   `json:"retry_count"`
	KnownURL        string                `json:"known_url,omitempty"`
}

// IntentMutation describes one journal change for an operation: write Payload, or delete it when Delete is set.
// RequireAbsent makes the write fail if an intent for OperationID already exists.
type IntentMutation struct {
	OperationID   string
	Payload       *IntentPayload
	Delete        bool
	RequireAbsent bool
}
