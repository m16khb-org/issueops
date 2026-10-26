package issueopsremote

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/port"
)

type fakeRetitler struct {
	calls    int
	readback string
}

func (f *fakeRetitler) Retitle(_ context.Context, _, _, _, title string) (string, error) {
	f.calls++
	if f.readback != "" {
		return f.readback, nil
	}
	return title, nil
}

func TestRemoteRetitlePreviewsThenRequiresTheReadback(t *testing.T) {
	record := completedMergeRecord()
	run := func(r *fakeRetitler, title string, confirm bool) error {
		service := NewRemoteRetitleService(mergeRecordReader{record}, func(string) (port.IssueProviderRetitler, error) { return r, nil })
		_, err := service.Retitle(context.Background(), record.ID, "pr", title, confirm)
		return err
	}
	ok := &fakeRetitler{}
	if err := run(ok, "feat: x", false); err != nil || ok.calls != 0 {
		t.Fatalf("preview err=%v calls=%d", err, ok.calls)
	}
	if err := run(ok, "feat: x", true); err != nil || ok.calls != 1 {
		t.Fatalf("confirm err=%v calls=%d", err, ok.calls)
	}
	if err := run(&fakeRetitler{readback: "old"}, "feat: x", true); err == nil || !strings.Contains(err.Error(), "reads back") {
		t.Fatalf("readback err = %v", err)
	}
	if err := run(ok, "a\nb", false); err == nil || !strings.Contains(err.Error(), "one line") {
		t.Fatalf("multi-line err = %v", err)
	}
	if err := run(ok, "token ghp_abcdefghijklmnopqrstuvwxyz0123456789AB", false); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("secret title err = %v", err)
	}
}
