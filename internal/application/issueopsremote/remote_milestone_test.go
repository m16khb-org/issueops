package issueopsremote

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/port"
)

type fakeMilestoneSetter struct {
	calls    int
	readback string
}

func (f *fakeMilestoneSetter) SetMilestone(_ context.Context, _, _, _, title string) (string, error) {
	f.calls++
	if f.readback != "" {
		return f.readback, nil
	}
	return title, nil
}

func TestRemoteMilestonePreviewsThenRequiresTheReadback(t *testing.T) {
	record := completedMergeRecord()
	run := func(f *fakeMilestoneSetter, title string, confirm bool) error {
		service := NewRemoteMilestoneService(mergeRecordReader{record}, func(string) (port.IssueProviderMilestoneSetter, error) { return f, nil })
		_, err := service.SetMilestone(context.Background(), record.ID, "pr", title, confirm)
		return err
	}
	ok := &fakeMilestoneSetter{}
	if err := run(ok, "v1.1", false); err != nil || ok.calls != 0 {
		t.Fatalf("preview err=%v calls=%d", err, ok.calls)
	}
	if err := run(ok, "v1.1", true); err != nil || ok.calls != 1 {
		t.Fatalf("confirm err=%v calls=%d", err, ok.calls)
	}
	if err := run(&fakeMilestoneSetter{readback: "v1.0"}, "v1.1", true); err == nil || !strings.Contains(err.Error(), "reads back") {
		t.Fatalf("readback err = %v", err)
	}
	if err := run(ok, " ", false); err == nil || !strings.Contains(err.Error(), "--set") {
		t.Fatalf("empty err = %v", err)
	}
}
