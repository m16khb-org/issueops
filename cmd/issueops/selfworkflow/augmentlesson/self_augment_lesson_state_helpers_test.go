package augmentlesson

import (
	"encoding/json"
	"time"

	application "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
)

type lessonTestDeps struct {
	IssueOpsRoot    func() string
	PrintJSON       func(any) error
	SelectCandidate func() *augmentcontract.Candidate
}

func SaveSelfAugmentLesson(req augmentcontract.LessonRequest, deps lessonTestDeps) (augmentcontract.LessonResult, error) {
	return application.SaveLesson(req, application.SaveLessonDeps{
		IssueOpsRoot: deps.IssueOpsRoot, SelectCandidate: deps.SelectCandidate,
		Now: time.Now,
		Encode: func(snapshot augmentcontract.LessonStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: StateWrite, StateDir: StateDir, Prune: StatePrunePrefix,
	})
}
