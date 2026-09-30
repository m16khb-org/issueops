package daemon

import (
	"context"
	"errors"
	"io"
	daemoncontract "issueops/internal/contract/daemon"
	daemondomain "issueops/internal/domain/daemon"
	"time"
)

const readyTimeout = 15 * time.Second

type Starter struct {
	Context     context.Context
	CheckStatus func() daemoncontract.Status
	Paths       func() (daemoncontract.Paths, error)
	EnsureDir   func(string) error
	AcquireLock func(daemoncontract.Paths) (io.Closer, error)
	Remove      func(string) error
	Executable  func() (string, error)
	StartDaemon func(string, daemoncontract.Paths) error
	Wait        func(context.Context, daemoncontract.Paths, time.Duration) (daemoncontract.Status, error)
}
type Waiter struct {
	Now          func() time.Time
	SleepContext func(context.Context, time.Duration) error
	CheckStatus  func() daemoncontract.Status
}

func (deps Starter) Run() (daemoncontract.Status, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return daemoncontract.Status{}, err
	}

	if status := deps.CheckStatus(); daemondomain.IsReady(status) {
		return status, nil
	} else if daemondomain.BlocksStart(status) {
		return status, errors.New(status.Code + ": " + status.Message)
	}
	paths, err := deps.Paths()
	if err != nil {
		return daemoncontract.Status{}, err
	}
	if err := deps.EnsureDir(paths.Dir); err != nil {
		return daemoncontract.Status{}, err
	}
	lock, err := deps.AcquireLock(paths)
	if err != nil {
		// 다른 launcher가 시작 중일 수 있다. 잠시 기다린다.
		if status, waitErr := deps.Wait(ctx, paths, readyTimeout); waitErr == nil && daemondomain.IsReady(status) {
			return status, nil
		}
		return daemoncontract.Status{OK: false, Running: false, Paths: paths, Message: err.Error()}, err
	}
	defer func() {
		_ = lock.Close()
		_ = deps.Remove(paths.Lock)
	}()
	if status := deps.CheckStatus(); daemondomain.IsReady(status) {
		return status, nil
	} else if daemondomain.BlocksStart(status) {
		return status, errors.New(status.Code + ": " + status.Message)
	}
	if err := ctx.Err(); err != nil {
		return daemoncontract.Status{}, err
	}
	exe, err := deps.Executable()
	if err != nil {
		return daemoncontract.Status{}, err
	}
	if err := deps.StartDaemon(exe, paths); err != nil {
		return daemoncontract.Status{OK: false, Paths: paths, Message: err.Error()}, err
	}
	return deps.Wait(ctx, paths, readyTimeout)
}

func (deps Waiter) Run(ctx context.Context, paths daemoncontract.Paths, timeout time.Duration) (daemoncontract.Status, error) {
	deadline := deps.Now().Add(timeout)
	var last daemoncontract.Status
	for deps.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		last = deps.CheckStatus()
		if daemondomain.IsReady(last) {
			return last, nil
		}
		if daemondomain.BlocksStart(last) {
			return last, errors.New(last.Code + ": " + last.Message)
		}
		if err := deps.SleepContext(ctx, 50*time.Millisecond); err != nil {
			return last, err
		}
	}
	if last.Paths.Dir == "" {
		last.Paths = paths
	}
	last.Message = "daemon did not become ready before timeout"
	return last, errors.New(last.Message)
}
