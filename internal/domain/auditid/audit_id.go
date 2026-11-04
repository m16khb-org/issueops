// Package auditid generates identifiers for command audit records.
package auditid

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync/atomic"
	"time"
)

var sequence atomic.Uint64

// Generate returns a unique audit id of the form
// audit-<UTC timestamp>-<process sequence>-<FNV-32a hash of workspaceRoot,
// cwd, and argv>. The sequence is process-wide and safe for concurrent use.
func Generate(workspaceRoot, cwd string, argv []string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(workspaceRoot))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cwd))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(argv, "\x00")))
	return fmt.Sprintf("audit-%s-%016x-%08x", time.Now().UTC().Format("20060102T150405.000000000Z"), sequence.Add(1), h.Sum32())
}
