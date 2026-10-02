package verification

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	domain "issueops/internal/domain/selfverify"
)

func TestRunCaptureDrainsConcurrentStreamsWithinAllocationBound(t *testing.T) {
	for _, size := range []int{8 << 20, 32 << 20} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			root := t.TempDir()
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			step := Run(root, "capture", 30*time.Second, "", 32<<10, os.Args[0],
				"-test.run=^TestCaptureHelperProcess$", "--", strconv.Itoa(size), "0")
			runtime.ReadMemStats(&after)
			allocated := after.TotalAlloc - before.TotalAlloc
			t.Logf("bytes_per_stream=%d allocated=%d", size, allocated)
			if !step.OK || step.StdoutBytes != size || step.StderrBytes != size ||
				!step.StdoutTruncated || !step.StderrTruncated {
				t.Fatalf("capture metadata: %+v", step)
			}
			for _, stream := range []struct{ got, tail string }{
				{step.Stdout, "stdout-tail"}, {step.Stderr, "stderr-tail"},
			} {
				if len(stream.got) > 32<<10 || !strings.HasSuffix(stream.got, stream.tail) {
					t.Fatalf("capture suffix: bytes=%d tail=%q", len(stream.got), stream.tail)
				}
				marker, tail, ok := strings.Cut(stream.got, "\n")
				want := fmt.Sprintf("[truncated: original_bytes=%d omitted_bytes=%d]", size, size-len(tail))
				if !ok || marker != want {
					t.Fatalf("marker=%q want=%q", marker, want)
				}
			}
			if allocated > 4<<20 {
				t.Errorf("capture allocations scale with output: %d", allocated)
			}
		})
	}
}

func TestRunCaptureMatchesBudgetOnFailureAndUnlimitedOutput(t *testing.T) {
	const size = 8192
	for _, budget := range []int{-1, 0, 1, 64, size, size + 1} {
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			step := Run(t.TempDir(), "failure", 30*time.Second, "", budget, os.Args[0],
				"-test.run=^TestCaptureHelperProcess$", "--", strconv.Itoa(size), "7")
			if step.OK || step.Error != "exit status 7" {
				t.Fatalf("exit contract: %+v", step)
			}
			for _, stream := range []struct {
				got       string
				truncated bool
				bytes     int
				tail      string
			}{
				{step.Stdout, step.StdoutTruncated, step.StdoutBytes, "stdout-tail"},
				{step.Stderr, step.StderrTruncated, step.StderrBytes, "stderr-tail"},
			} {
				input := strings.Repeat("x", size-len(stream.tail)) + stream.tail
				want, truncated, total := domain.BudgetCommandOutput(input, budget)
				if stream.got != want || stream.truncated != truncated || stream.bytes != total {
					t.Fatalf("stream differs from formatter: bytes=%d truncated=%v", stream.bytes, stream.truncated)
				}
			}
		})
	}
}

func TestRunCaptureAllocatesForSmallOutputRatherThanBudget(t *testing.T) {
	root := t.TempDir()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	step := Run(root, "small capture", 30*time.Second, "", 4<<20, os.Args[0],
		"-test.run=^TestCaptureHelperProcess$", "--", "64", "0")
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("bytes_per_stream=64 allocated=%d", allocated)
	if !step.OK || step.StdoutBytes != 64 || step.StderrBytes != 64 ||
		step.StdoutTruncated || step.StderrTruncated ||
		step.Stdout != strings.Repeat("x", 64-len("stdout-tail"))+"stdout-tail" ||
		step.Stderr != strings.Repeat("x", 64-len("stderr-tail"))+"stderr-tail" {
		t.Fatalf("small capture contract: %+v", step)
	}
	if allocated > 1<<20 {
		t.Errorf("small output allocated the capture budget: %d", allocated)
	}
}

func TestCaptureHelperProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--" || i+2 >= len(os.Args) {
			continue
		}
		size, err := strconv.Atoi(os.Args[i+1])
		if err != nil {
			os.Exit(64)
		}
		code, err := strconv.Atoi(os.Args[i+2])
		if err != nil {
			os.Exit(64)
		}
		var wg sync.WaitGroup
		for _, stream := range []struct {
			writer io.Writer
			tail   string
		}{{os.Stdout, "stdout-tail"}, {os.Stderr, "stderr-tail"}} {
			wg.Go(func() {
				chunk := []byte(strings.Repeat("x", 8192))
				for remaining := size - len(stream.tail); remaining > 0; {
					n := min(remaining, len(chunk))
					if _, err := stream.writer.Write(chunk[:n]); err != nil {
						os.Exit(65)
					}
					remaining -= n
				}
				if _, err := io.WriteString(stream.writer, stream.tail); err != nil {
					os.Exit(65)
				}
			})
		}
		wg.Wait()
		os.Exit(code)
	}
}
