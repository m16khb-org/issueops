package candidateexport

import domain "issueops/internal/domain/selfverify"

func SelfVerificationCandidateCatalog() []SelfVerificationCandidate { return domain.CandidateCatalog() }
