package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli/benchmarkartifact"
	"issueops/cmd/issueops/issueopscli/benchmarkcmd"
	benchmark "issueops/internal/adapter/issueops/benchmark"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/issueopsbenchmark"
	"os"
	"time"
)

func newBenchmarkCommand() benchmarkcmd.Command {
	return benchmarkcmd.Command{Service: &app.Service{Files: benchmark.Files{Stdin: os.Stdin}, Runs: benchmark.Store{Directory: statestore.StateDir()}, Now: time.Now, Artifact: benchmarkartifact.FromFixture}}
}
