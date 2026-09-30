package augmentlesson

import (
	"encoding/json"
	"time"

	application "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

type lessonTestDeps struct {
	IssueOpsRoot    func() string
	PrintJSON       func(any) error
	SelectCandidate func() *augmentcontract.SelfAugmentCandidate
}

func SaveSelfAugmentLesson(req augmentcontract.SelfAugmentLessonRequest, deps lessonTestDeps) (augmentcontract.SelfAugmentLessonResult, error) {
	return application.SaveLesson(req, application.SaveLessonDeps{
		IssueOpsRoot: deps.IssueOpsRoot, SelectCandidate: deps.SelectCandidate,
		Now: time.Now,
		Encode: func(snapshot augmentcontract.SelfAugmentLessonStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: StateWrite, StateDir: StateDir, Prune: StatePrunePrefix,
	})
}

func StateKeySlug(s string) string { return domain.StateKeySlug(s) }
