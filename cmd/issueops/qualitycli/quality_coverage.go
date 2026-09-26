package qualitycli

import (
	"context"
	"errors"
)

type CoverageEffects struct {
	Run         func(string, func(context.Context, string) (string, error), func() (string, error)) (string, error)
	Execute     func(context.Context, string) (string, error)
	CacheBase   func() (string, error)
	Fingerprint func(string) (string, error)
	CachePath   func(string, func() (string, error)) (string, error)
}

var coverageEffects CoverageEffects

func ConfigureCoverage(effects CoverageEffects) { coverageEffects = effects }

var executeGoTestCoverage = func(ctx context.Context, root string) (string, error) {
	if coverageEffects.Execute == nil {
		return "", errors.New("quality coverage adapter is not configured")
	}
	return coverageEffects.Execute(ctx, root)
}
var coverageCacheBase = func() (string, error) {
	if coverageEffects.CacheBase == nil {
		return "", errors.New("quality coverage adapter is not configured")
	}
	return coverageEffects.CacheBase()
}

func runGoTestCoverage(root string) (string, error) {
	if coverageEffects.Run == nil {
		return "", errors.New("quality coverage adapter is not configured")
	}
	return coverageEffects.Run(root, executeGoTestCoverage, coverageCacheBase)
}
func coverageFingerprint(root string) (string, error) {
	if coverageEffects.Fingerprint == nil {
		return "", errors.New("quality coverage adapter is not configured")
	}
	return coverageEffects.Fingerprint(root)
}
func coverageCachePath(root string) (string, error) {
	if coverageEffects.CachePath == nil {
		return "", errors.New("quality coverage adapter is not configured")
	}
	return coverageEffects.CachePath(root, coverageCacheBase)
}
