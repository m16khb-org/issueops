package state

// Result is the outcome of a state read or write for a single record in StateDir; Path is set when the record lives in a file.
type Result struct {
	OK       bool           `json:"ok"`
	StateDir string         `json:"state_dir"`
	Path     string         `json:"path,omitempty"`
	Record   RecordEnvelope `json:"record"`
}

// ListEntry is the metadata of one stored record without its content.
type ListEntry struct {
	Key           string `json:"key"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
	SchemaVersion int    `json:"schema_version"`
}

// ListResult is the outcome of listing records: the sorted keys and their metadata.
type ListResult struct {
	OK       bool        `json:"ok"`
	StateDir string      `json:"state_dir"`
	Keys     []string    `json:"keys"`
	Records  []ListEntry `json:"records"`
}

// PruneResult is the outcome of pruning records older than MaxAge. Without Confirm it is a DryRun: Pruned lists what would be deleted and DeletedKeys stays empty of real deletions.
type PruneResult struct {
	OK          bool        `json:"ok"`
	StateDir    string      `json:"state_dir"`
	MaxAge      string      `json:"max_age"`
	Cutoff      string      `json:"cutoff"`
	Confirm     bool        `json:"confirm"`
	DryRun      bool        `json:"dry_run"`
	DeletedKeys []string    `json:"deleted_keys"`
	Pruned      []ListEntry `json:"pruned"`
	KeptKeys    []string    `json:"kept_keys"`
	Kept        []ListEntry `json:"kept"`
}

// DoctorIssue is one problem found while checking the state store, identified by Code and Severity and located by Path and optional Key.
type DoctorIssue struct {
	Path     string `json:"path"`
	Key      string `json:"key,omitempty"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// DoctorResult is the outcome of a state store health check: how many records were checked, which are valid, and the Issues found. Healthy is true when there are none.
type DoctorResult struct {
	OK        bool          `json:"ok"`
	Healthy   bool          `json:"healthy"`
	StateDir  string        `json:"state_dir"`
	Checked   int           `json:"checked"`
	ValidKeys []string      `json:"valid_keys"`
	Valid     []ListEntry   `json:"valid"`
	Issues    []DoctorIssue `json:"issues"`
}

// StoreMaintainResult reports the maintenance of one store root: the WAL size before and after truncation, whether a checkpoint ran, and the files whose permissions were corrected.
type StoreMaintainResult struct {
	Dir              string   `json:"dir"`
	WALBytesBefore   int64    `json:"wal_bytes_before"`
	WALBytesAfter    int64    `json:"wal_bytes_after"`
	Checkpointed     bool     `json:"checkpointed"`
	PermissionsFixed []string `json:"permissions_fixed,omitempty"`
}

// MaintainResult is the outcome of maintaining every existing store root; Skipped names roots that have no store.
type MaintainResult struct {
	OK      bool                  `json:"ok"`
	Roots   []StoreMaintainResult `json:"roots"`
	Skipped []string              `json:"skipped,omitempty"`
}
