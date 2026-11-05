// Package guard is the domain layer for the change guard: it classifies changed paths and file contents,
// flags secrets, risky patterns, and look-alike symbols, and decides whether a check blocks. It is pure
// analysis over observations the caller supplies.
package guard

import (
	"path/filepath"
	"sort"
	"strings"

	guardcontract "issueops/internal/contract/guard"
)

// FileObservation is one file handed to Analyze: its path, its content, and whether the content was
// read. Files that could not be read are still judged by path.
type FileObservation struct {
	Path    string
	Content string
	Read    bool
}

// Analysis is the outcome of Analyze: the sorted findings, their severity counts, and OK, which is true
// when no finding blocks.
type Analysis struct {
	OK       bool
	Findings []guardcontract.Finding
	Summary  guardcontract.Summary
}

// Mode names the file selection a check request uses: "all", "staged", or "files". A request that
// selects nothing defaults to "staged".
func Mode(request guardcontract.CheckRequest) string {
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
	if testPath(rel) {
		kinds.test = true
	} else if SourcePath(rel) {
		kinds.prod = true
	}
	if contractSurfacePath(rel) {
		kinds.contractSurface = true
	}
	if strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.Contains(strings.ToLower(rel), "golden") {
		kinds.golden = true
	}
}

// Analyze judges the observed files and returns deduplicated findings sorted by severity, file, line, and rule.
// Secret-like paths block and are skipped from content checks; other findings cover repeated symbols from
// existingSymbols, production changes without tests, and contract surface changes without golden updates.
func Analyze(files []FileObservation, existingSymbols map[string][]string) Analysis {
	result := Analysis{Findings: []guardcontract.Finding{}}
	var kinds changeKinds
	for _, file := range files {
		rel := file.Path
		if SecretLikePath(rel) {
			result.Findings = append(result.Findings, guardcontract.Finding{
				Severity: "block", Rule: "secret-like-path", File: rel,
				Message: "Secret-like paths must not be committed or analyzed as ordinary source.",
			})
			continue
		}
		kinds.observe(rel)
		if file.Read {
			result.Findings = append(result.Findings, fileFindings(rel, file.Content, existingSymbols)...)
		}
	}
	if kinds.prod && !kinds.test {
		result.Findings = append(result.Findings, guardcontract.Finding{
			Severity: "warn", Rule: "prod-change-without-test",
			Message: "Production source changed without a changed test file; verify this is documentation/config-only or add focused coverage.",
		})
	}
	if kinds.contractSurface && !kinds.golden {
		result.Findings = append(result.Findings, guardcontract.Finding{
			Severity: "warn", Rule: "contract-surface-without-golden",
			Message: "CLI/MCP/adapter contract surface changed without a golden/testdata update.",
		})
	}
	result.Findings = dedupeFindings(result.Findings)
	sort.Slice(result.Findings, func(i, j int) bool {
		return findingLess(result.Findings[i], result.Findings[j])
	})
	result.Summary = summarizeFindings(result.Findings)
	result.OK = result.Summary.Block == 0
	return result
}

// findingLess orders by severity rank, then file, line and rule.
func findingLess(a, b guardcontract.Finding) bool {
	if severityRank(a.Severity) != severityRank(b.Severity) {
		return severityRank(a.Severity) < severityRank(b.Severity)
	}
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Rule < b.Rule
}

func summarizeFindings(findings []guardcontract.Finding) guardcontract.Summary {
	var summary guardcontract.Summary
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
