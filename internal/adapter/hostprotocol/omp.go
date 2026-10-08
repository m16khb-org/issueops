package hostprotocol

// ompSessionIDEnv carries the main omp session id to shell children of the omp process.
const ompSessionIDEnv = "ISSUEOPS_OMP_SESSION_ID"

// OmpLifecycleExtension returns the one canonical omp lifecycle module for a harness binary.
//
// omp emits session_switch for /new, fork, and resume, and its session_compact
// payload has no accepted flag. Subagents share the omp process, so only the
// main agent may export its session id into process.env.
func OmpLifecycleExtension(binPath string) string {
	return lifecycleExtension(binPath, contract{
		SchemaVersion: 1,
		Events: map[string]rule{
			"session_start":   {Subcommand: "session-start"},
			"session_switch":  {Subcommand: "session-start"},
			"session_compact": {Subcommand: "post-compact"},
		},
		Message: message{CustomType: "issueops:project-docs", Display: false, TriggerTurn: false},
		Warning: "issueops lifecycle hook failed",
		SessionEnv: &sessionEnv{
			Variable:  ompSessionIDEnv,
			AgentKind: "main",
			Events:    []string{"session_start", "session_switch"},
		},
	})
}
