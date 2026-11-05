package trace

// Context is the parsed W3C traceparent correlation (trace id, parent span id, and trace flags) carried into recorded events.
type Context struct {
	TraceID  string `json:"trace_id"`
	ParentID string `json:"parent_id"`
	Flags    string `json:"flags"`
}
