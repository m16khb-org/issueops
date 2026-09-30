package selfaugment

import (
	"strings"
	"time"

	model "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

type SaveLessonDeps struct {
	IssueOpsRoot    func() string
	SelectCandidate func() *model.SelfAugmentCandidate
	Now             func() time.Time
	Encode          func(model.SelfAugmentLessonStateSnapshot) ([]byte, error)
	Write           func(string, string) (state.StateResult, error)
	StateDir        func() string
	Prune           func(string, time.Duration, int, bool) (state.StatePruneResult, error)
}

func SaveLesson(req model.SelfAugmentLessonRequest, deps SaveLessonDeps) (model.SelfAugmentLessonResult, error) {
	root := "."
	if deps.IssueOpsRoot != nil {
		root = deps.IssueOpsRoot()
	}
	candidateID := strings.TrimSpace(req.CandidateID)
	if candidateID == "" && deps.SelectCandidate != nil {
		if candidate := deps.SelectCandidate(); candidate != nil {
			candidateID = candidate.ID
		}
	}
	result, err := domain.PrepareLesson(req, root, candidateID)
	if err != nil {
		return result, err
	}
	now := deps.Now().UTC()
	result.GeneratedAt = now.Format(time.RFC3339Nano)
	snapshot, key := domain.LessonSnapshot(result, req.StateKey, now)

	b, err := deps.Encode(snapshot)
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return result, err
	}
	state, err := deps.Write(key, string(b))
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: deps.StateDir(), Error: err.Error()}
		return result, err
	}
	result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	_, _ = deps.Prune(domain.LessonStateKeyPrefix, domain.LessonStateMaxAge, domain.LessonStateMaxRecords, true)
	return result, nil
}
