package cmux

import (
	"context"
	"fmt"

	contract "issueops/internal/contract/cmux"
	model "issueops/internal/contract/issueops"
)

type Client interface {
	Preflight(context.Context, contract.PreflightRequest) (contract.PreflightResult, error)
	CreateWorkspace(context.Context, contract.CreateRequest) (contract.CreatedWorkspace, error)
	Send(context.Context, contract.SendRequest) (contract.SendReceipt, error)
}
type Directory interface {
	Path() string
	SamePath(string) bool
	Pin() (DirectoryPin, error)
}
type DirectoryPin interface {
	Matches(Directory) bool
	Close()
}
type RootEnvironment struct {
	Read      func(string, string) (model.IssueOpsRecord, error)
	Directory func(string) (Directory, error)
	Getwd     func() (string, error)
	GitTop    func(string) (string, error)
}
type MutationError struct {
	Phase     string
	Ambiguous bool
	Cause     error
}

func (err *MutationError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("cmux %s failed: %v", err.Phase, err.Cause)
}

func (err *MutationError) Unwrap() error { return err.Cause }
