package issueops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// processHelperCollectTimeout bounds how long a failed barrier waits for the
// helpers it released to exit. It only delays a test that already failed.
const processHelperCollectTimeout = time.Minute

// processHelperExit is what one helper process left behind when it exited.
type processHelperExit struct {
	worker int
	output string
	err    error
}

// awaitHelperReadyMarkers waits until every ready marker exists and then
// returns nil without reading exits. When readyTimeout passes first, it calls
// release so helpers waiting at the gate go on, collects one exit per marker
// within collectTimeout, and returns an error with each worker's index, exit
// error, and full combined output. A helper that fails before its marker (for
// example while opening the store) is reported with its own error text, and
// the parent never fails while helpers it started are still running unless
// the collect bound is spent.
func awaitHelperReadyMarkers(markers []string, readyTimeout time.Duration, release func() error, exits <-chan processHelperExit, collectTimeout time.Duration) error {
	deadline := time.Now().Add(readyTimeout)
	for {
		missing := []string{}
		for _, marker := range markers {
			if _, err := os.Stat(marker); err != nil {
				missing = append(missing, filepath.Base(marker))
			}
		}
		if len(missing) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return helperBarrierFailure(missing, len(markers), release, exits, collectTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func helperBarrierFailure(missing []string, workers int, release func() error, exits <-chan processHelperExit, collectTimeout time.Duration) error {
	var report strings.Builder
	fmt.Fprintf(&report, "workers did not reach ready barrier in time; missing=%v", missing)
	if err := release(); err != nil {
		fmt.Fprintf(&report, "\nrelease gate: %v", err)
	}
	collected := receiveHelperExits(exits, workers, collectTimeout)
	for _, exit := range collected {
		writeHelperExit(&report, exit)
	}
	writeHelpersStillRunning(&report, collected, workers, collectTimeout)
	return errors.New(report.String())
}

// collectHelperExits receives one exit from every worker before it judges
// any of them, so a failed helper never ends the test while its siblings still
// run. It returns the exits indexed by worker and, when a worker failed or did
// not exit within collectTimeout, one error naming each such worker with its
// exit error and full combined output.
func collectHelperExits(exits <-chan processHelperExit, workers int, collectTimeout time.Duration) ([]processHelperExit, error) {
	collected := receiveHelperExits(exits, workers, collectTimeout)
	var report strings.Builder
	for _, exit := range collected {
		if exit.err != nil {
			writeHelperExit(&report, exit)
		}
	}
	writeHelpersStillRunning(&report, collected, workers, collectTimeout)
	byWorker := make([]processHelperExit, workers)
	for _, exit := range collected {
		byWorker[exit.worker] = exit
	}
	if report.Len() > 0 {
		return byWorker, fmt.Errorf("helper processes failed:%s", report.String())
	}
	return byWorker, nil
}

// receiveHelperExits receives up to workers exits within timeout and returns
// them sorted by worker index.
func receiveHelperExits(exits <-chan processHelperExit, workers int, timeout time.Duration) []processHelperExit {
	collected := make([]processHelperExit, 0, workers)
	deadline := time.After(timeout)
	for len(collected) < workers {
		select {
		case exit := <-exits:
			collected = append(collected, exit)
		case <-deadline:
			return sortedHelperExits(collected)
		}
	}
	return sortedHelperExits(collected)
}

func sortedHelperExits(exits []processHelperExit) []processHelperExit {
	slices.SortFunc(exits, func(a, b processHelperExit) int { return a.worker - b.worker })
	return exits
}

func writeHelperExit(report *strings.Builder, exit processHelperExit) {
	status := "exited cleanly"
	if exit.err != nil {
		status = "exit error: " + exit.err.Error()
	}
	fmt.Fprintf(report, "\nworker %d %s\ncombined output:\n%s", exit.worker, status, exit.output)
}

func writeHelpersStillRunning(report *strings.Builder, collected []processHelperExit, workers int, timeout time.Duration) {
	for worker := range workers {
		if !slices.ContainsFunc(collected, func(exit processHelperExit) bool { return exit.worker == worker }) {
			fmt.Fprintf(report, "\nworker %d still running after %v", worker, timeout)
		}
	}
}

func TestHelperBarrierTimeoutReportsEveryHelperExit(t *testing.T) {
	dir := t.TempDir()
	markers := []string{filepath.Join(dir, "ready-worker-0"), filepath.Join(dir, "ready-worker-1")}
	exits := make(chan processHelperExit, len(markers))

	// Worker 0 reaches the barrier and finishes once the gate opens.
	if err := os.WriteFile(markers[0], []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		<-released
		exits <- processHelperExit{worker: 0, output: "worker 0 finished after the gate\n"}
	}()
	// Worker 1 fails to open the store before it writes its marker.
	exits <- processHelperExit{
		worker: 1,
		output: "--- FAIL: TestIssueOpsDelegationAcceptProcessHelper\n    injected-open-failure\n",
		err:    errors.New("exit status 1"),
	}

	release := func() error {
		close(released)
		return nil
	}
	err := awaitHelperReadyMarkers(markers, 100*time.Millisecond, release, exits, 5*time.Second)
	if err == nil {
		t.Fatal("barrier passed with a helper that never wrote its marker")
	}
	for _, want := range []string{
		"ready-worker-1",
		"worker 1",
		"exit status 1",
		"injected-open-failure",
		"worker 0",
		"worker 0 finished after the gate",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("barrier error lacks %q:\n%v", want, err)
		}
	}
	select {
	case <-released:
	default:
		t.Error("barrier failed without releasing the gate")
	}
	if len(exits) != 0 {
		t.Errorf("barrier left %d helper exits uncollected", len(exits))
	}
}

func TestHelperBarrierTimeoutNamesHelpersStillRunning(t *testing.T) {
	dir := t.TempDir()
	markers := []string{filepath.Join(dir, "ready-worker-0")}
	exits := make(chan processHelperExit)

	err := awaitHelperReadyMarkers(markers, 10*time.Millisecond, func() error { return nil }, exits, 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "worker 0") || !strings.Contains(err.Error(), "still running") {
		t.Fatalf("barrier error does not name the helper that is still running: %v", err)
	}
}

func TestCollectHelperExitsWaitsForEveryHelperBeforeReporting(t *testing.T) {
	exits := make(chan processHelperExit)
	go func() {
		exits <- processHelperExit{
			worker: 1,
			output: "--- FAIL: helper\n    injected-accept-failure\n",
			err:    errors.New("exit status 1"),
		}
		time.Sleep(50 * time.Millisecond)
		exits <- processHelperExit{worker: 0, output: "accepted child-0\n"}
	}()

	collected, err := collectHelperExits(exits, 2, 5*time.Second)
	if err == nil {
		t.Fatal("collect accepted a failed helper")
	}
	for _, want := range []string{"worker 1", "exit status 1", "injected-accept-failure"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("collect error lacks %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "worker 0") {
		t.Errorf("collect error reports the helper that succeeded:\n%v", err)
	}
	if len(collected) != 2 || collected[0].output != "accepted child-0\n" || collected[1].err == nil {
		t.Fatalf("collect returned before both helpers exited: %#v", collected)
	}
}

func TestCollectHelperExitsAcceptsCleanHelpers(t *testing.T) {
	exits := make(chan processHelperExit, 2)
	exits <- processHelperExit{worker: 1, output: "one\n"}
	exits <- processHelperExit{worker: 0, output: "zero\n"}

	collected, err := collectHelperExits(exits, 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if collected[0].output != "zero\n" || collected[1].output != "one\n" {
		t.Fatalf("exits not indexed by worker: %#v", collected)
	}
}
