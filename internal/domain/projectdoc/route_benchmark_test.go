package projectdoc

import (
	"strings"
	"testing"
)

func BenchmarkRouteDocsForTask(b *testing.B) {
	for _, task := range []string{
		"general",
		"implement test performance refactor ci pr",
		strings.Repeat("문서검토 implementation-helper ", 32) + "design ci",
	} {
		b.Run(task, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				RouteDocsForTask(task)
			}
		})
	}
}
