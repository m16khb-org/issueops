package looprun

import (
	"context"
	"errors"
	"io/fs"

	loopcontract "issueops/internal/contract/looprun"
	loopdomain "issueops/internal/domain/looprun"
)

type Store interface {
	WithLock(context.Context, string, func(context.Context) error) error
	Read(string) (loopcontract.LoopRun, error)
	Write(loopcontract.LoopRun) (loopcontract.LoopRun, error)
}

type Identity interface {
	NormalizeRepo(string) (string, error)
	NormalizeID(string) (string, error)
	NewID(repo, name string) string
}

type Clock interface{ Now() string }

type Service struct {
	Store         Store
	Identity      Identity
	Clock         Clock
	SchemaVersion int
}

func (service Service) Start(request loopcontract.StartLoopRequest) (loopcontract.LoopRun, error) {
	repo, err := service.Identity.NormalizeRepo(request.Repo)
	if err != nil {
		return loopcontract.LoopRun{OK: false}, err
	}
	prepared, err := loopdomain.PrepareStart(request)
	if err != nil {
		return loopcontract.LoopRun{OK: false}, err
	}
	loopID := service.Identity.NewID(repo, prepared.Name)
	var loop loopcontract.LoopRun
	err = service.Store.WithLock(context.Background(), loopID, func(context.Context) error {
		existing, readErr := service.Store.Read(loopID)
		if readErr == nil {
			if err := loopdomain.Resume(existing); err != nil {
				return err
			}
			loop = existing
			return nil
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		loop = loopdomain.New(loopID, repo, prepared, service.Clock.Now(), service.SchemaVersion)
		loop, readErr = service.Store.Write(loop)
		return readErr
	})
	return loop, err
}

func (service Service) RecordAttempt(loopID string, request loopcontract.RecordAttemptRequest) (loopcontract.LoopRun, error) {
	loopID, err := service.Identity.NormalizeID(loopID)
	if err != nil {
		return loopcontract.LoopRun{OK: false}, err
	}
	prepared, err := loopdomain.PrepareAttempt(request)
	if err != nil {
		return loopcontract.LoopRun{OK: false, ID: loopID}, err
	}
	var loop loopcontract.LoopRun
	err = service.Store.WithLock(context.Background(), loopID, func(context.Context) error {
		var readErr error
		loop, readErr = service.Store.Read(loopID)
		if readErr != nil {
			return readErr
		}
		next, transitionErr := loopdomain.ApplyAttempt(loop, prepared, service.Clock.Now())
		if transitionErr != nil {
			return transitionErr
		}
		loop, readErr = service.Store.Write(next)
		return readErr
	})
	return loop, err
}

func (service Service) Stop(loopID string, success bool, reason string) (loopcontract.LoopRun, error) {
	loopID, err := service.Identity.NormalizeID(loopID)
	if err != nil {
		return loopcontract.LoopRun{OK: false}, err
	}
	var loop loopcontract.LoopRun
	err = service.Store.WithLock(context.Background(), loopID, func(context.Context) error {
		var readErr error
		loop, readErr = service.Store.Read(loopID)
		if readErr != nil {
			return readErr
		}
		next, transitionErr := loopdomain.Stop(loop, success, reason, service.Clock.Now())
		if transitionErr != nil {
			return transitionErr
		}
		loop, readErr = service.Store.Write(next)
		return readErr
	})
	return loop, err
}
