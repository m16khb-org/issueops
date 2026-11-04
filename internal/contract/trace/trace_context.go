package trace

type Context struct {
	TraceID  string `json:"trace_id"`
	ParentID string `json:"parent_id"`
	Flags    string `json:"flags"`
}
