package issueops

import (
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestCleanupStopTargetsRequireCurrentPreviewReceipts(t *testing.T) {
	parent := model.CleanupWorkspaceProcess{PID: 20, StartedAt: "first", Executable: "/bin/sh"}
	child := model.CleanupWorkspaceProcess{PID: 21, StartedAt: "child", Executable: "/bin/sleep"}
	replaced := parent
	replaced.StartedAt = "replacement"
	for _, tc := range []struct {
		name               string
		occupants, preview []model.CleanupWorkspaceProcess
		ancestry           map[int][]int
		excluded           map[int]bool
		want               int
		changed            bool
		refused            bool
	}{
		{name: "exact receipt", occupants: []model.CleanupWorkspaceProcess{parent}, preview: []model.CleanupWorkspaceProcess{parent}, ancestry: map[int][]int{100: {1}}, want: 1},
		{name: "PID reused", occupants: []model.CleanupWorkspaceProcess{replaced}, preview: []model.CleanupWorkspaceProcess{parent}, ancestry: map[int][]int{100: {1}}, changed: true, refused: true},
		{name: "live bound ancestor", occupants: []model.CleanupWorkspaceProcess{parent, child}, preview: []model.CleanupWorkspaceProcess{parent}, ancestry: map[int][]int{100: {1}, 21: {20, 1}}, want: 2},
		{name: "absent ancestor", occupants: []model.CleanupWorkspaceProcess{child}, preview: []model.CleanupWorkspaceProcess{parent}, ancestry: map[int][]int{100: {1}, 21: {20, 1}}, changed: true, refused: true},
		{name: "requester ancestor", occupants: []model.CleanupWorkspaceProcess{parent}, preview: []model.CleanupWorkspaceProcess{parent}, ancestry: map[int][]int{100: {20, 1}}, refused: true},
		{name: "explicit exclusion", occupants: []model.CleanupWorkspaceProcess{parent}, preview: []model.CleanupWorkspaceProcess{parent}, ancestry: map[int][]int{100: {1}}, excluded: map[int]bool{20: true}, refused: true},
		{name: "requester unobserved", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CleanupStopTargets(tc.occupants, tc.ancestry, 100, tc.preview, tc.excluded)
			if (err != nil) != tc.refused || errors.Is(err, ErrCleanupOccupancyChanged) != tc.changed || len(got) != tc.want {
				t.Fatalf("targets=%+v err=%v", got, err)
			}
		})
	}
}
