package issueops

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	authorityapp "issueops/internal/application/authority"
	"issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
	"issueops/internal/testsupport/authoritytest"
)

func liveTestVerifier() authorityport.ActorVerifier {
	return authorityapp.New(nil, authoritytest.ProcessInspectorFunc(func(_ context.Context, receipt issueops.NativeProcessReceipt) (string, issueops.NativeProcessReceipt, error) {
		return NativeProcessStatusLive, receipt, nil
	}), nil, nil, nil, nil)
}

// liveFixtureProcessEnv carries the fixture process pid to helper processes so
// they observe the same live session as their parent test process.
const liveFixtureProcessEnv = "ISSUEOPS_TEST_LIVE_FIXTURE_PID"

// liveFixtureReceipt is the real receipt of a live process shared by every
// helper process, so native verification of fixture holders observes a live
// session. Unix has PID 1; Windows has no such process this user can read, so
// the first test process publishes its own pid through the environment and
// helper processes it spawns observe that pid.
var liveFixtureReceipt = sync.OnceValue(func() issueops.NativeProcessReceipt {
	pid := 1
	if runtime.GOOS == "windows" {
		pid = os.Getpid()
		if inherited, err := strconv.Atoi(os.Getenv(liveFixtureProcessEnv)); err == nil && inherited > 0 {
			pid = inherited
		} else if err := os.Setenv(liveFixtureProcessEnv, strconv.Itoa(pid)); err != nil {
			panic(fmt.Sprintf("publish live fixture pid: %v", err))
		}
	}
	receipt, err := ObserveNativeProcessReceipt(pid)
	if err != nil {
		panic(fmt.Sprintf("observe live fixture receipt: %v", err))
	}
	return receipt
})
