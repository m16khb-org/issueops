package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/benchmarkartifact"
	"issueops/cmd/issueops/issueopscli/benchmarkcmd"
	benchmarkadapter "issueops/internal/adapter/issueops/benchmark"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/issueopsbenchmark"
	"os"
	"time"
)

func testBenchmarkCommand() benchmarkcmd.Command {
	return benchmarkcmd.Command{Service: &app.Service{Files: benchmarkadapter.Files{Stdin: os.Stdin}, Runs: benchmarkadapter.Store{Directory: statestore.StateDir()}, Now: time.Now, Artifact: benchmarkartifact.FromFixture}}
}
