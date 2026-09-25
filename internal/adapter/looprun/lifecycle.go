package looprun

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	loopapp "issueops/internal/application/looprun"
	loopruncontract "issueops/internal/contract/looprun"
	looprundomain "issueops/internal/domain/looprun"
)

func Start(req loopruncontract.StartLoopRequest) (loopruncontract.LoopRun, error) {
	return loopService().Start(req)
}

func RecordAttempt(loopID string, req loopruncontract.RecordAttemptRequest) (loopruncontract.LoopRun, error) {
	return loopService().RecordAttempt(loopID, req)
}

func Stop(loopID string, success bool, reason string) (loopruncontract.LoopRun, error) {
	return loopService().Stop(loopID, success, reason)
}

func loopService() loopapp.Service {
	return loopapp.Service{Store: loopStore{}, Identity: loopIdentity{}, Clock: loopClock{}, SchemaVersion: LoopRunCurrentSchemaVersion}
}

type loopStore struct{}

func (loopStore) WithLock(ctx context.Context, id string, fn func(context.Context) error) error {
	return withLoopLock(ctx, id, fn)
}
func (loopStore) Read(id string) (loopruncontract.LoopRun, error) { return ReadLoop(id) }
func (loopStore) Write(loop loopruncontract.LoopRun) (loopruncontract.LoopRun, error) {
	return writeLoop(loop)
}

type loopIdentity struct{}

func (loopIdentity) NormalizeRepo(repo string) (string, error) { return normalizeRepo(repo) }
func (loopIdentity) NormalizeID(id string) (string, error)     { return normalizeLoopID(id) }
func (loopIdentity) NewID(repo, name string) string            { return newLoopID(repo, name) }

type loopClock struct{}

func (loopClock) Now() string { return timestampNow() }

func Status(loopID string) (loopruncontract.StatusResult, error) {
	loop, err := ReadLoop(loopID)
	if err != nil {
		return loopruncontract.StatusResult{OK: false}, err
	}
	return looprundomain.Status(loop), nil
}

func withLoopLock(ctx context.Context, loopID string, fn func(context.Context) error) error {
	if _, err := normalizeLoopID(loopID); err != nil {
		return err
	}
	db, err := openStore()
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}

func normalizeRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return abs, nil
}
