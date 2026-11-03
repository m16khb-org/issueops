// Package osprocess holds the Windows process-tree primitives that replace
// Unix process groups and signals. Unix adapters keep their own Setpgid and
// kill(-pgid) code; this package has no Unix implementation.
package osprocess
