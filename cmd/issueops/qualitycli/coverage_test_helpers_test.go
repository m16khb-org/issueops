package qualitycli

import coverage "issueops/internal/adapter/outbound/quality"

func init() {
	ConfigureSNRScanner(coverage.ComputeCodeSNR)
	ConfigureSourceCollectors(coverage.CollectBranchFunctions, coverage.CollectAuditItems, coverage.CollectPioneerCoverage)
	ConfigureCoverage(CoverageEffects{
		Run:         coverage.RunGoTestCoverage,
		Execute:     coverage.ExecuteGoTestCoverage,
		CacheBase:   coverage.DefaultCoverageCacheBase,
		Fingerprint: coverage.CoverageFingerprint,
		CachePath:   coverage.CoverageCachePath,
	})
}
