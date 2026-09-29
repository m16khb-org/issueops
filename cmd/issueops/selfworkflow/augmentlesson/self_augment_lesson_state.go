package augmentlesson

import (
	"encoding/json"
	"time"

	"issueops/cmd/issueops/selfworkflow/model"
	application "issueops/internal/application/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

type Deps struct {
	IssueOpsRoot    func() string
	PrintJSON       func(any) error
	SelectCandidate func() *model.SelfAugmentCandidate
}

func SaveSelfAugmentLesson(req model.SelfAugmentLessonRequest, deps Deps) (model.SelfAugmentLessonResult, error) {
	return application.SaveLesson(req, application.SaveLessonDeps{
		IssueOpsRoot: deps.IssueOpsRoot, SelectCandidate: deps.SelectCandidate,
		Now: time.Now,
		Encode: func(snapshot model.SelfAugmentLessonStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: StateWrite, StateDir: StateDir, Prune: StatePrunePrefix,
	})
}

func StateKeySlug(s string) string { return domain.StateKeySlug(s) }
