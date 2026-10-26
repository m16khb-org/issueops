package issueopsremote

import (
	"context"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type fakeLabeler struct {
	calls    [][]string
	readback []string
}

func (f *fakeLabeler) AddLabels(_ context.Context, _, _, _ string, labels []string) ([]string, error) {
	f.calls = append(f.calls, labels)
	return f.readback, nil
}

func runLabel(t *testing.T, labeler *fakeLabeler, labels []string, confirm bool) (model.RemoteLabelResult, error) {
	t.Helper()
	record := completedMergeRecord()
	service := NewRemoteLabelService(mergeRecordReader{record}, func(string) (port.IssueProviderLabeler, error) { return labeler, nil })
	return service.AddLabels(context.Background(), record.ID, "pr", labels, confirm)
}

func TestRemoteLabelPreviewsThenAddsAndRequiresTheReadback(t *testing.T) {
	labeler := &fakeLabeler{readback: []string{"enhancement", "cli"}}
	preview, err := runLabel(t, labeler, []string{"cli, cli"}, false)
	if err != nil || !preview.OK || len(labeler.calls) != 0 || len(preview.Add) != 1 {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	added, err := runLabel(t, labeler, []string{"cli"}, true)
	if err != nil || !added.Added || len(labeler.calls) != 1 {
		t.Fatalf("added = %+v, err = %v", added, err)
	}
	missing := &fakeLabeler{readback: []string{"enhancement"}}
	if _, err := runLabel(t, missing, []string{"cli"}, true); err == nil || !strings.Contains(err.Error(), "not read back") {
		t.Fatalf("missing readback err = %v", err)
	}
	if _, err := runLabel(t, labeler, nil, false); err == nil || !strings.Contains(err.Error(), "--add LABEL") {
		t.Fatalf("empty err = %v", err)
	}
}
