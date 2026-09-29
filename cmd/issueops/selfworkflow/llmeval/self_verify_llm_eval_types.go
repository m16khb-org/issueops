package llmeval

import (
	augmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

type SelfVerifyLLMEvalOptions = selfverifycontract.LLMEvalOptions

type SelfVerifyLLMEvalResult = augmentcontract.SelfVerifyLLMEvalResult
