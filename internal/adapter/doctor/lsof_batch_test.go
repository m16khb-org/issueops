package doctor

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const lsofListenersFixture = `p901
f13
n*:49244
f15
n*:49244
p912
f10
n*:7000
f12
n127.0.0.1:5000
p1072
f25
n127.0.0.1:7335
f26
n[::1]:7335
p2000
f9
n127.0.0.1:7000
`

func TestParseLsofListenersMapsPortsToFirstPid(t *testing.T) {
	got := parseLsofListeners([]byte(lsofListenersFixture))
	want := map[int]int{49244: 901, 7000: 912, 5000: 912, 7335: 1072}
	if len(got) != len(want) {
		t.Fatalf("listeners = %v, want %v", got, want)
	}
	for port, pid := range want {
		if got[port] != pid {
			t.Fatalf("port %d -> pid %d, want %d (first reported pid wins)", port, got[port], pid)
		}
	}
	if got := parseLsofListeners([]byte("garbage\npabc\nn*:80\nn:x\n")); len(got) != 0 {
		t.Fatalf("malformed output produced listeners %v", got)
	}
}

func TestParseLsofFDCountsCountsFilesPerPidAndOmitsUnlistedPids(t *testing.T) {
	out := "p912\nf10\nf11\nf12\np1072\nf25\np3000\n"
	got := parseLsofFDCounts([]byte(out))
	if len(got) != 3 || got[912] != 3 || got[1072] != 1 {
		t.Fatalf("counts = %v", got)
	}
	if count, listed := got[3000]; !listed || count != 0 {
		t.Fatalf("pid listed without files = %d, %v, want 0 and listed", count, listed)
	}
	if _, listed := got[4000]; listed {
		t.Fatalf("pid absent from the output must stay absent: %v", got)
	}
}

func TestCountGatewayFDsBatchCallsLsofTwiceForAnyNumberOfPorts(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		if slices.Contains(args, "-sTCP:LISTEN") {
			return []byte(lsofListenersFixture), nil
		}
		// 7000 and 5000 share pid 912; the missing pid 1072 is not listed,
		// which lsof also reports with exit status 1 next to partial output.
		return []byte("p912\nf1\nf2\nf3\n"), errors.New("exit status 1")
	}
	got := countGatewayFDsBatch(t.Context(), []int{5000, 7000, 7335, 9999}, run)
	if len(calls) != 2 {
		t.Fatalf("lsof calls = %v, want one listener scan and one fd listing", calls)
	}
	if len(calls[1]) != 3 || calls[1][0] != "-p" || calls[1][1] != "912,1072" {
		t.Fatalf("fd listing args = %v, want each listener pid once", calls[1])
	}
	if got[5000].Err != nil || got[5000].Count != 3 || got[7000].Err != nil || got[7000].Count != 3 {
		t.Fatalf("ports sharing a pid = %+v %+v", got[5000], got[7000])
	}
	if got[7335].Err == nil || !strings.Contains(got[7335].Err.Error(), "fd listing failed for pid 1072") {
		t.Fatalf("pid missing from the fd listing = %+v", got[7335])
	}
	if got[9999].Err == nil || got[9999].Err.Error() != "no listener process found" {
		t.Fatalf("port without listener = %+v", got[9999])
	}
}

func TestCountGatewayFDsBatchDegradesEveryPortWhenLsofFails(t *testing.T) {
	failing := func(context.Context, ...string) ([]byte, error) { return nil, errors.New("lsof: not found") }
	got := countGatewayFDsBatch(t.Context(), []int{7001, 7002}, failing)
	for _, port := range []int{7001, 7002} {
		if got[port].Err == nil || got[port].Count != 0 || !strings.Contains(got[port].Err.Error(), "listener pid lookup failed") {
			t.Fatalf("port %d = %+v, want unavailable", port, got[port])
		}
	}
	run := func(_ context.Context, args ...string) ([]byte, error) {
		if slices.Contains(args, "-sTCP:LISTEN") {
			return []byte(lsofListenersFixture), nil
		}
		return nil, errors.New("exit status 1")
	}
	got = countGatewayFDsBatch(t.Context(), []int{7335, 5000}, run)
	for _, port := range []int{7335, 5000} {
		if got[port].Err == nil || !strings.Contains(got[port].Err.Error(), "fd listing failed") {
			t.Fatalf("port %d = %+v, want fd listing failure", port, got[port])
		}
	}
	if got := countGatewayFDsBatch(t.Context(), nil, failing); len(got) != 0 {
		t.Fatalf("no ports must not call lsof: %v", got)
	}
}

func TestCountGatewayFDsViaLsofSeesThisProcessListener(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not installed")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port
	got := CountGatewayFDsViaLsof(t.Context(), []int{port})[port]
	if got.Err != nil || got.Count < 3 {
		t.Fatalf("own listener = %+v, want a listener with at least stdio and the socket", got)
	}
}
