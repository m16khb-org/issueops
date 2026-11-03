//go:build unix

package executioncmd

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// processCommandLine reads one process's argv. Only an Omo bun candidate
// needs it, so the ancestry snapshot itself stays a single ps call.
func processCommandLine(pid int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-ww", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
