package actorreceipt

import (
	"flag"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestReceiptFlagsMustNameALiveAncestor(t *testing.T) {
	live := []issueopscontract.NativeProcessReceipt{{PID: 42, StartedAt: "2026-10-23T00:00:00Z", Executable: "/usr/bin/claude"}}
	cases := map[string]struct {
		args         []string
		keepAncestry bool
	}{
		"no receipt flags": {nil, true},
		"matching receipt": {[]string{"--session-pid", "42", "--session-started-at", "2026-10-23T00:00:00Z", "--session-executable", "/usr/bin/claude"}, true},
		"foreign pid":      {[]string{"--session-pid", "43", "--session-started-at", "2026-10-23T00:00:00Z", "--session-executable", "/usr/bin/claude"}, false},
		"stale start time": {[]string{"--session-pid", "42", "--session-started-at", "2026-10-22T00:00:00Z", "--session-executable", "/usr/bin/claude"}, false},
		"other executable": {[]string{"--session-pid", "42", "--session-started-at", "2026-10-23T00:00:00Z", "--session-executable", "/usr/bin/codex"}, false},
		"partial receipt":  {[]string{"--session-pid", "42"}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := flag.NewFlagSet("record", flag.ContinueOnError)
			receipt := Add(fs)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			got, err := receipt.Observer(func() ([]issueopscontract.NativeProcessReceipt, error) { return live, nil })()
			if err != nil {
				t.Fatal(err)
			}
			if tc.keepAncestry != (len(got) == 1) {
				t.Fatalf("ancestry = %v, want kept=%v", got, tc.keepAncestry)
			}
		})
	}
}
