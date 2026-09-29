package daemon

type Paths struct {
	Dir    string `json:"dir"`
	Socket string `json:"socket"`
	PID    string `json:"pid_file"`
	Lock   string `json:"lock_file"`
	Log    string `json:"log_file"`
}

type InstanceRecord struct {
	PID              int    `json:"pid"`
	ProcessStartTime string `json:"process_start_time"`
	Executable       string `json:"executable"`
	InstanceNonce    string `json:"instance_nonce"`
	BuildSHA         string `json:"build_sha"`
	ProtocolVersion  string `json:"protocol_version"`
	Generation       string `json:"generation"`
}

type Status struct {
	OK                bool            `json:"ok"`
	Running           bool            `json:"running"`
	Reachable         bool            `json:"reachable"`
	IdentityVerified  bool            `json:"identity_verified"`
	ActiveConnections int             `json:"active_connections"`
	MaxConnections    int             `json:"max_connections"`
	Accepting         bool            `json:"accepting"`
	Draining          bool            `json:"draining"`
	PID               int             `json:"pid,omitempty"`
	Code              string          `json:"code"`
	Paths             Paths           `json:"paths"`
	Instance          *InstanceRecord `json:"instance,omitempty"`
	Message           string          `json:"message,omitempty"`
}

const (
	StatusReady              = "ready"
	StatusStopped            = "stopped"
	StatusSocketUnreachable  = "socket_unreachable"
	StatusIdentityMismatch   = "instance_identity_mismatch"
	StatusInstanceUnreadable = "instance_record_unreadable"
)

type ProcessIdentity struct {
	StartTime            string
	Executable           string
	ExecutablePathStable bool
}

type IdentityResponse struct {
	OK                bool           `json:"ok"`
	Instance          InstanceRecord `json:"instance"`
	ActiveConnections int            `json:"active_connections"`
	MaxConnections    int            `json:"max_connections"`
	Accepting         bool           `json:"accepting"`
	Draining          bool           `json:"draining"`
}
