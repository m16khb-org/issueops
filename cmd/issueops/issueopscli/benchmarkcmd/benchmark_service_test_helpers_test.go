package benchmarkcmd

import (
	"issueops/cmd/issueops/issueopscli/benchmarkartifact"
	benchmarkadapter "issueops/internal/adapter/issueops/benchmark"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/issueopsbenchmark"
	"os"
	"time"
)

func testBenchmarkCommand() Command {
	return Command{Service: &app.Service{Files: benchmarkadapter.Files{Stdin: os.Stdin}, Runs: benchmarkadapter.Store{Directory: statestore.StateDir()}, Now: time.Now, Artifact: benchmarkartifact.FromFixture}}
}
