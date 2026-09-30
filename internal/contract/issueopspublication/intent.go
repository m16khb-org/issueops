package issueopspublication

const RemoteIntentKind = "remote_pr_create"

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

type IntentMutation struct {
	OperationID   string
	Payload       *IntentPayload
	Delete        bool
	RequireAbsent bool
}
