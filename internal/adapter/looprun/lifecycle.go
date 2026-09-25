package looprun

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	loopruncontract "issueops/internal/contract/looprun"
	looprundomain "issueops/internal/domain/looprun"
	"path/filepath"
	"strings"
)

func Start(req loopruncontract.StartLoopRequest) (loopruncontract.LoopRun, error) {
	repo, err := normalizeRepo(req.Repo)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	prepared, err := looprundomain.PrepareStart(req)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	loopID := newLoopID(repo, prepared.Name)
	var loop loopruncontract.LoopRun
	err = withLoopLock(context.Background(), loopID, func(context.Context) error {
		existing, readErr := ReadLoop(loopID)
		if readErr == nil {
			if err := looprundomain.Resume(existing); err != nil {
				return err
			}
			loop = existing
			return nil
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		now := timestampNow()
		loop = looprundomain.New(loopID, repo, prepared, now, LoopRunCurrentSchemaVersion)
		var writeErr error
		loop, writeErr = writeLoop(loop)
		return writeErr
	})
	return loop, err
}

func RecordAttempt(loopID string, req loopruncontract.RecordAttemptRequest) (loopruncontract.LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	prepared, err := looprundomain.PrepareAttempt(req)
	if err != nil {
		return loopruncontract.LoopRun{OK: false, ID: loopID}, err
	}
	var loop loopruncontract.LoopRun
	err = withLoopLock(context.Background(), loopID, func(context.Context) error {
		var readErr error
		loop, readErr = ReadLoop(loopID)
		if readErr != nil {
			return readErr
		}
		now := timestampNow()
		next, transitionErr := looprundomain.ApplyAttempt(loop, prepared, now)
		if transitionErr != nil {
			return transitionErr
		}
		loop = next
		var writeErr error
		loop, writeErr = writeLoop(loop)
		return writeErr
	})
	return loop, err
}

func Stop(loopID string, success bool, reason string) (loopruncontract.LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return loopruncontract.LoopRun{OK: false}, err
	}
	var loop loopruncontract.LoopRun
	err = withLoopLock(context.Background(), loopID, func(context.Context) error {
		var readErr error
		loop, readErr = ReadLoop(loopID)
		if readErr != nil {
			return readErr
		}
		now := timestampNow()
		next, transitionErr := looprundomain.Stop(loop, success, reason, now)
		if transitionErr != nil {
			return transitionErr
		}
		loop = next
		var writeErr error
		loop, writeErr = writeLoop(loop)
		return writeErr
	})
	return loop, err
}

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
