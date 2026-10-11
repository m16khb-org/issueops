package selfverify

import (
	"reflect"
	"testing"
)

func failing(seed int64, labels ...string) Run {
	run := Run{Seed: seed}
	for _, label := range labels {
		run.Checks = append(run.Checks, Check{Label: label, OK: false})
	}
	return run
}

func TestClassifyFailureDistinguishesPatterns(t *testing.T) {
	pass := Run{Seed: 1, Checks: []Check{{Label: "go test", OK: true}}}
	cases := []struct {
		name        string
		failed, tot int
		runs        []Run
		class       string
	}{
		{"no failures", 0, 2, []Run{pass}, ""},
		{"failures without details", 1, 1, []Run{pass}, "unknown"},
		{"some seeds fail", 1, 2, []Run{failing(1, "go test"), pass}, "intermittent"},
		{"single observation", 1, 1, []Run{failing(1, "go test")}, "single_failure_observation"},
		{"same step everywhere", 2, 2, []Run{failing(1, "go test"), failing(2, "go test")}, "deterministic"},
		{"different steps", 2, 2, []Run{failing(1, "go test"), failing(2, "gofmt")}, "mixed"},
	}
	for _, tc := range cases {
		class, reason, clusters := ClassifyFailure(tc.failed, tc.tot, tc.runs)
		if class != tc.class {
			t.Fatalf("%s: class=%q", tc.name, class)
		}
		if (class == "") != (reason == "") {
			t.Fatalf("%s: reason=%q", tc.name, reason)
		}
		if (class == "" || class == "unknown") && clusters != nil {
			t.Fatalf("%s: clusters=%v", tc.name, clusters)
		}
	}
}

func TestFailureClustersSortStepsAndSeeds(t *testing.T) {
	clusters := FailureClusters([]Run{failing(9, "gofmt", "go test"), failing(3, "gofmt")})
	if len(clusters) != 2 || clusters[0].Step != "go test" || clusters[1].Step != "gofmt" {
		t.Fatalf("clusters=%+v", clusters)
	}
	if !reflect.DeepEqual(clusters[1].Seeds, []int64{3, 9}) || clusters[1].Count != 2 {
		t.Fatalf("gofmt cluster=%+v", clusters[1])
	}
	if got := FailureClusters(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty result must be non-nil empty, got %#v", got)
	}
}
