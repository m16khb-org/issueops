package issueopsapp

import (
	fixturecontract "issueops/internal/contract/toolconformance"

	"context"
	"fmt"
	failurecause "issueops/internal/adapter/failurecause"
	"issueops/internal/adapter/hostprotocol"
	app "issueops/internal/application/toolconformance"
	"os"
	"strings"
	"time"

	"issueops/cmd/issueops/contractcli"
	"issueops/internal/adapter/hostprobe"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/toolconformance"
	"issueops/internal/port"
)

func runToolConformanceLive(ctx context.Context, request contractcli.LiveRequest) (fixturecontract.BenchmarkReport, error) {
	binary, err := os.Executable()
	if err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	models, err := conformanceModelOverrides(request.Models)
	if err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	descriptors := make([]fixturecontract.ToolDescriptor, 0, len(mcpcatalog.AdvertisedTools()))
	for _, tool := range mcpcatalog.AdvertisedTools() {
		descriptors = append(descriptors, fixturecontract.ToolDescriptor{Name: tool.Name, InputSchema: tool.InputSchema})
	}
	runners := toolConformanceRunners(binary)
	return app.RunLiveBenchmark(ctx, app.LiveBenchmarkRequest{
		Hosts: request.Hosts, Models: models,
		Profile: request.Profile, Only: request.Only, TargetCompleted: request.TargetCompleted,
		MaxAttemptsPerCase: request.MaxAttemptsPerCase, HarnessBinary: binary, Previous: request.Previous,
	}, descriptors, app.LiveBenchmarkDependencies{Runners: runners, Now: time.Now, Token: toolconformance.RandomToken, LoadManifest: newConformanceFixtures().LoadManifest, Classify: failurecause.Classify})
}

func toolConformanceRunners(binary string) map[string]port.HostProbeRunner {
	return map[string]port.HostProbeRunner{
		"codex":  hostprobe.NewCodexRunner(binary, hostprobe.Dependencies{}),
		"claude": hostprobe.NewClaudeRunner(binary, hostprobe.Dependencies{}),
		"omo":    hostprobe.NewOmoRunner(binary, hostprotocol.OmoLifecycleExtension(binary), hostprobe.Dependencies{}, hostprotocol.OmoLifecycleExtension),
	}
}

func conformanceModelOverrides(values []string) (map[string]string, error) {
	models := map[string]string{}
	for _, value := range values {
		host, model, found := strings.Cut(value, "=")
		host, model = strings.TrimSpace(host), strings.TrimSpace(model)
		if !found || host == "" || model == "" {
			return nil, fmt.Errorf("invalid model override %q", value)
		}
		if _, exists := models[host]; exists {
			return nil, fmt.Errorf("duplicate model override for %s", host)
		}
		models[host] = model
	}
	return models, nil
}
