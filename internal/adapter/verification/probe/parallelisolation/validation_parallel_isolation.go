// Package parallelisolation is the self-verification probe that checks
// parallel workers get isolated temporary roots, state directories, and
// artifacts.
package parallelisolation

import (
	selfverify "issueops/internal/contract/selfverify"

	"sync"
	"time"
)

// Validate runs three concurrent probes of the built binary and fails when
// any probe errors or two workers share a temp root, state directory, or
// artifact path. Failures are reported in the returned StepResult.
func Validate(binary, root string, seed int64) selfverify.StepResult {
	return validateParallelTempIsolationWithDeps(binary, root, seed, parallelIsolationValidationDeps{})
}

func validateParallelTempIsolationWithDeps(binary, root string, seed int64, deps parallelIsolationValidationDeps) selfverify.StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	const workers = 3
	results := make(chan parallelIsolationProbe, workers)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() { results <- deps.runProbe(binary, root, seed, worker) })
	}
	wg.Wait()
	close(results)

	probes := []parallelIsolationProbe{}
	for probe := range results {
		probes = append(probes, probe)
	}
	return parallelIsolationResult(started, workers, probes)
}
