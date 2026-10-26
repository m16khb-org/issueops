package issueopsremote

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/port"
)

type fakeAssigner struct {
	calls    int
	readback []string
}

func (f *fakeAssigner) AddAssignees(context.Context, string, string, string, []string) ([]string, error) {
	f.calls++
	return f.readback, nil
}

func TestRemoteAssignAddsBareUsernamesAndRequiresTheReadback(t *testing.T) {
	record := completedMergeRecord()
	run := func(a *fakeAssigner, users []string, confirm bool) error {
		service := NewRemoteAssignService(mergeRecordReader{record}, func(string) (port.IssueProviderAssigner, error) { return a, nil })
		_, err := service.AddAssignees(context.Background(), record.ID, "pr", users, confirm)
		return err
	}
	ok := &fakeAssigner{readback: []string{"qa-user", "kim"}}
	if err := run(ok, []string{"kim"}, false); err != nil || ok.calls != 0 {
		t.Fatalf("preview err=%v calls=%d", err, ok.calls)
	}
	if err := run(ok, []string{"kim"}, true); err != nil || ok.calls != 1 {
		t.Fatalf("confirm err=%v calls=%d", err, ok.calls)
	}
	if err := run(&fakeAssigner{readback: []string{"qa-user"}}, []string{"kim"}, true); err == nil || !strings.Contains(err.Error(), "not read back") {
		t.Fatalf("readback err = %v", err)
	}
	// glab reads a leading - or ! as removal; @ and + are syntax, not names.
	for _, user := range []string{"-kim", "!kim", "+kim", "@kim"} {
		if err := run(ok, []string{user}, false); err == nil || !strings.Contains(err.Error(), "bare username") {
			t.Fatalf("%s err = %v", user, err)
		}
	}
}
