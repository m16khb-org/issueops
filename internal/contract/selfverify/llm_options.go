package selfverify

// LLMEvalOptions configures the optional LLM evaluation of a self-verification
// result: whether it runs, its mode (advisory or gate, defaulting to advisory),
// and the score to judge against.
type LLMEvalOptions struct {
	Enabled     bool
	Mode        string
	TargetScore float64
}
