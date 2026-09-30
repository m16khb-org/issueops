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

func Analyze(files []FileObservation, existingSymbols map[string][]string) Analysis {
	result := Analysis{Findings: []guardcontract.GuardFinding{}}
	hasProdChange := false
	hasTestChange := false
	hasContractSurfaceChange := false
	hasGoldenChange := false
	for _, file := range files {
		rel := file.Path
		if SecretLikePath(rel) {
			result.Findings = append(result.Findings, guardcontract.GuardFinding{
				Severity: "block", Rule: "secret-like-path", File: rel,
				Message: "Secret-like paths must not be committed or analyzed as ordinary source.",
			})
			continue
		}
		if TestPath(rel) {
			hasTestChange = true
		} else if SourcePath(rel) {
			hasProdChange = true
		}
		if ContractSurfacePath(rel) {
			hasContractSurfaceChange = true
		}
		if strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.Contains(strings.ToLower(rel), "golden") {
			hasGoldenChange = true
		}
		if file.Read {
			result.Findings = append(result.Findings, FileFindings(rel, file.Content, existingSymbols)...)
		}
	}
	if hasProdChange && !hasTestChange {
		result.Findings = append(result.Findings, guardcontract.GuardFinding{
			Severity: "warn", Rule: "prod-change-without-test",
			Message: "Production source changed without a changed test file; verify this is documentation/config-only or add focused coverage.",
		})
	}
	if hasContractSurfaceChange && !hasGoldenChange {
		result.Findings = append(result.Findings, guardcontract.GuardFinding{
			Severity: "warn", Rule: "contract-surface-without-golden",
			Message: "CLI/MCP/adapter contract surface changed without a golden/testdata update.",
		})
	}
	result.Findings = DedupeFindings(result.Findings)
	sort.Slice(result.Findings, func(i, j int) bool {
		if SeverityRank(result.Findings[i].Severity) != SeverityRank(result.Findings[j].Severity) {
			return SeverityRank(result.Findings[i].Severity) < SeverityRank(result.Findings[j].Severity)
		}
		if result.Findings[i].File != result.Findings[j].File {
			return result.Findings[i].File < result.Findings[j].File
		}
		if result.Findings[i].Line != result.Findings[j].Line {
			return result.Findings[i].Line < result.Findings[j].Line
		}
		return result.Findings[i].Rule < result.Findings[j].Rule
	})
	for _, finding := range result.Findings {
		switch finding.Severity {
		case "block":
			result.Summary.Block++
		case "warn":
			result.Summary.Warn++
		case "review":
			result.Summary.Review++
		default:
			result.Summary.Info++
		}
	}
	result.OK = result.Summary.Block == 0
	return result
}
