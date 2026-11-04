package selfaugment

import (
	"strings"
	"time"

	model "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

// SaveLessonDeps supplies the root, candidate selection, clock, encoding, state
// writer and pruning that SaveLesson uses.
type SaveLessonDeps struct {
	IssueOpsRoot    func() string
	SelectCandidate func() *model.Candidate
	Now             func() time.Time
	Encode          func(model.LessonStateSnapshot) ([]byte, error)
	Write           func(string, string) (state.Result, error)
	StateDir        func() string
	Prune           func(string, time.Duration, int, bool) (state.PruneResult, error)
}

// SaveLesson records a lesson about a self-augmentation candidate and checkpoints
// it in the state store. The candidate is the request's, or the selected one when
// the request names none. Pruning old lessons afterwards is best effort and its
// error is ignored; encode and write failures are returned and recorded in the
// result's StateCheckpoint.
func SaveLesson(req model.LessonRequest, deps SaveLessonDeps) (model.LessonResult, error) {
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
		result.StateCheckpoint = &model.StateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return result, err
	}
	state, err := deps.Write(key, string(b))
	if err != nil {
		result.OK = false
		result.StateCheckpoint = &model.StateCheckpoint{OK: false, Key: key, StateDir: deps.StateDir(), Error: err.Error()}
		return result, err
	}
	result.StateCheckpoint = &model.StateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	_, _ = deps.Prune(domain.LessonStateKeyPrefix, domain.LessonStateMaxAge, domain.LessonStateMaxRecords, true)
	return result, nil
}
