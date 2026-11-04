package state

type Result struct {
	OK       bool           `json:"ok"`
	StateDir string         `json:"state_dir"`
	Path     string         `json:"path,omitempty"`
	Record   RecordEnvelope `json:"record"`
}

type ListEntry struct {
	Key           string `json:"key"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
	SchemaVersion int    `json:"schema_version"`
}

type ListResult struct {
	OK       bool        `json:"ok"`
	StateDir string      `json:"state_dir"`
	Keys     []string    `json:"keys"`
	Records  []ListEntry `json:"records"`
}

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

type DoctorIssue struct {
	Path     string `json:"path"`
	Key      string `json:"key,omitempty"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type DoctorResult struct {
	OK        bool          `json:"ok"`
	Healthy   bool          `json:"healthy"`
	StateDir  string        `json:"state_dir"`
	Checked   int           `json:"checked"`
	ValidKeys []string      `json:"valid_keys"`
	Valid     []ListEntry   `json:"valid"`
	Issues    []DoctorIssue `json:"issues"`
}

type StoreMaintainResult struct {
	Dir              string   `json:"dir"`
	WALBytesBefore   int64    `json:"wal_bytes_before"`
	WALBytesAfter    int64    `json:"wal_bytes_after"`
	Checkpointed     bool     `json:"checkpointed"`
	PermissionsFixed []string `json:"permissions_fixed,omitempty"`
}

type MaintainResult struct {
	OK      bool                  `json:"ok"`
	Roots   []StoreMaintainResult `json:"roots"`
	Skipped []string              `json:"skipped,omitempty"`
}
