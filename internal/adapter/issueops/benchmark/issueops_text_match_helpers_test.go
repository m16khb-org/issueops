package benchmark

import domain "issueops/internal/domain/issueopsbenchmark"

func containsFold(s, needle string) bool { return domain.ContainsFold(s, needle) }
