package selfaugment

import contract "issueops/internal/contract/selfaugment"

func ResearchInfluences() []contract.SelfAugmentInfluence {
	return []contract.SelfAugmentInfluence{
		{Name: "Reflexion", Source: "https://arxiv.org/abs/2303.11366", Adopted: "scalar/test feedback is converted into reusable verbal lessons between cycles"},
		{Name: "Self-Refine", Source: "https://arxiv.org/abs/2303.17651", Adopted: "generate-feedback-refine is used inside candidate design and implementation retries"},
		{Name: "Voyager", Source: "https://arxiv.org/abs/2305.16291", Adopted: "automatic curriculum and skill-library thinking guide open-ended improvement selection"},
		{Name: "SWE-agent", Source: "https://arxiv.org/abs/2405.15793", Adopted: "agent-computer interface discipline: repository navigation, file edits, and tests are explicit loop surfaces"},
		{Name: "AgentBench", Source: "https://arxiv.org/abs/2308.03688", Adopted: "multi-dimensional goal scoring replaces vague done/not-done loop exits"},
		{Name: "SWE-bench", Source: "https://arxiv.org/abs/2310.06770", Adopted: "real repository issue resolution is the model for improvement candidates"},
		{Name: "LangGraph", Source: "https://docs.langchain.com/oss/python/langgraph/overview", Adopted: "durable state, human oversight, and recovery are kept as design constraints"},
		{Name: "AutoGen", Source: "https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/human-in-the-loop.html", Adopted: "termination conditions and max-turn safeguards inspire explicit score gates"},
		{Name: "DSPy optimizers", Source: "https://github.com/stanfordnlp/dspy/blob/main/docs/docs/learn/optimization/optimizers.md", Adopted: "metric-first optimization shapes candidate scoring and regression checks"},
		{Name: "OpenAI Evals", Source: "https://github.com/openai/evals", Adopted: "evaluation artifacts must be reusable and rights-safe before promotion"},
	}
}
