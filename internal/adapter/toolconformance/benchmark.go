package toolconformance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	toolconformanceapp "issueops/internal/application/toolconformance"
	"issueops/internal/port"
)

type LiveBenchmarkRequest = toolconformanceapp.LiveBenchmarkRequest

type LiveBenchmarkDependencies struct {
	Runners map[string]port.HostProbeRunner
	Now     func() time.Time
	Token   func() string
}

func RunLiveBenchmark(ctx context.Context, request LiveBenchmarkRequest, descriptors []ToolDescriptor, deps LiveBenchmarkDependencies) (BenchmarkReport, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Token == nil {
		deps.Token = randomToken
	}
	return toolconformanceapp.RunLiveBenchmark(ctx, request, descriptors, toolconformanceapp.LiveBenchmarkDependencies{
		Runners: deps.Runners, Now: deps.Now, Token: deps.Token, LoadManifest: LoadManifest, Classify: ClassifyFailureCause,
	})
}

func BuildEpisodePrompt(fixture Fixture, profile string) (string, string) {
	return toolconformanceapp.BuildEpisodePrompt(fixture, profile)
}

func DiagnosticSignature(classification Classification, diagnostics []Diagnostic) string {
	return toolconformanceapp.DiagnosticSignature(classification, diagnostics)
}

func validEvidenceID(value string) bool { return toolconformanceapp.ValidEvidenceID(value) }

func randomToken() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		sum := sha256.Sum256([]byte(time.Now().UTC().String()))
		return hex.EncodeToString(sum[:16])
	}
	return hex.EncodeToString(value)
}
