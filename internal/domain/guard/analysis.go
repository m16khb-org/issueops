package guard

import (
	"path/filepath"
	"sort"
	"strings"

	guardcontract "issueops/internal/contract/guard"
)

type FileObservation struct {
	Path    string
	Content string
	Read    bool
}

type Analysis struct {
	OK       bool
	Findings []guardcontract.GuardFinding
	Summary  guardcontract.GuardSummary
}

func Mode(request guardcontract.GuardCheckRequest) string {
	if request.All {
		return "all"
	}
	if request.Staged {
		return "staged"
	}
	if len(request.Files) > 0 {
		return "files"
	}
	return "staged"
}

// changeKinds records which kinds of change the analyzed files contain.
type changeKinds struct {
	prod, test, contractSurface, golden bool
}

func (kinds *changeKinds) observe(rel string) {
	if TestPath(rel) {
		kinds.test = true
	} else if SourcePath(rel) {
		kinds.prod = true
	}
	if ContractSurfacePath(rel) {
		kinds.contractSurface = true
	}
	if strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.Contains(strings.ToLower(rel), "golden") {
		kinds.golden = true
	}
}

func Analyze(files []FileObservation, existingSymbols map[string][]string) Analysis {
	result := Analysis{Findings: []guardcontract.GuardFinding{}}
	var kinds changeKinds
	for _, file := range files {
		rel := file.Path
		if SecretLikePath(rel) {
			result.Findings = append(result.Findings, guardcontract.GuardFinding{
				Severity: "block", Rule: "secret-like-path", File: rel,
				Message: "Secret-like paths must not be committed or analyzed as ordinary source.",
			})
			continue
		}
		kinds.observe(rel)
		if file.Read {
			result.Findings = append(result.Findings, FileFindings(rel, file.Content, existingSymbols)...)
		}
	}
	if kinds.prod && !kinds.test {
		result.Findings = append(result.Findings, guardcontract.GuardFinding{
			Severity: "warn", Rule: "prod-change-without-test",
			Message: "Production source changed without a changed test file; verify this is documentation/config-only or add focused coverage.",
		})
	}
	if kinds.contractSurface && !kinds.golden {
		result.Findings = append(result.Findings, guardcontract.GuardFinding{
			Severity: "warn", Rule: "contract-surface-without-golden",
			Message: "CLI/MCP/adapter contract surface changed without a golden/testdata update.",
		})
	}
	result.Findings = DedupeFindings(result.Findings)
	sort.Slice(result.Findings, func(i, j int) bool {
		return findingLess(result.Findings[i], result.Findings[j])
	})
	result.Summary = summarizeFindings(result.Findings)
	result.OK = result.Summary.Block == 0
	return result
}

// findingLess orders by severity rank, then file, line and rule.
func findingLess(a, b guardcontract.GuardFinding) bool {
	if SeverityRank(a.Severity) != SeverityRank(b.Severity) {
		return SeverityRank(a.Severity) < SeverityRank(b.Severity)
	}
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Rule < b.Rule
}

func summarizeFindings(findings []guardcontract.GuardFinding) guardcontract.GuardSummary {
	var summary guardcontract.GuardSummary
	for _, finding := range findings {
		switch finding.Severity {
		case "block":
			summary.Block++
		case "warn":
			summary.Warn++
		case "review":
			summary.Review++
		default:
			summary.Info++
		}
	}
	return summary
}
