// Package issueopsview orchestrates the read-only `issueops view` use case: it
// reads a live cycle record, falls back to the outcome cleanup or prune retained when the
// record is gone, and shapes the answer without writing anything.
package issueopsview

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	issueopsviewcontract "issueops/internal/contract/issueopsview"
	issueopsdomain "issueops/internal/domain/issueops"
	issueopsviewdomain "issueops/internal/domain/issueopsview"
)

// Ports are the reads the view service needs. ReadRecord answers a missing
// record with an error that wraps fs.ErrNotExist; RetainedOutcome answers a
// missing retained row with false and a nil error. Next is the same `issueops
// next` judgment the CLI prints and WorktreePresent says whether a worktree
// directory is a git worktree that exists; the graph view alone uses them.
type Ports struct {
	ReadRecord      func(ctx context.Context, stateRoot, id string) (issueopsviewcontract.Record, error)
	RetainedOutcome func(ctx context.Context, stateRoot, id string) (issueopsviewcontract.CycleOutcome, bool, error)
	Next            func(ctx context.Context, stateRoot, cwd, id string) (issueopsviewcontract.NextResult, error)
	WorktreePresent func(ctx context.Context, root string) bool
	Now             func() time.Time
}

// Service renders cycle views. Construct it with NewService.
type Service struct {
	ports Ports
}

// NewService returns a Service over ports. It does not check them: View fails
// with an error when a port it needs is nil.
func NewService(ports Ports) *Service {
	return &Service{ports: ports}
}

// View renders the named view of cycle id. A live record wins; the retained
// outcome is read only when the record read fails with fs.ErrNotExist, and any
// other record error is returned as is. The timeline view reads that outcome's
// timeline; the graph view needs the live record and answers an error that
// names the retired cycle instead. cwd is passed to `next` for the graph view
// and is unused by the timeline view.
func (service *Service) View(ctx context.Context, stateRoot, cwd, id, view string) (issueopsviewcontract.Result, error) {
	failed := issueopsviewcontract.Result{OK: false}
	if service == nil || service.ports.ReadRecord == nil || service.ports.RetainedOutcome == nil || service.ports.Now == nil {
		return failed, errDependencies
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return failed, errors.New("view requires --id")
	}
	if view != issueopsviewcontract.ViewTimeline && view != issueopsviewcontract.ViewGraph {
		return failed, fmt.Errorf("--view must be %s or %s", issueopsviewcontract.ViewTimeline, issueopsviewcontract.ViewGraph)
	}
	if view == issueopsviewcontract.ViewGraph && (service.ports.Next == nil || service.ports.WorktreePresent == nil) {
		return failed, errDependencies
	}

	record, recordErr := service.ports.ReadRecord(ctx, stateRoot, id)
	if recordErr == nil {
		if record.ID != "" {
			id = record.ID
		}
		if view == issueopsviewcontract.ViewGraph {
			return service.graph(ctx, stateRoot, cwd, id, record), nil
		}
		timeline := issueopsdomain.BuildCycleTimeline(record)
		return issueopsviewcontract.Result{
			OK: true, View: view, ID: id, Source: issueopsviewcontract.SourceRecord,
			GeneratedAt: service.generatedAt(), Timeline: &timeline,
		}, nil
	}
	if !errors.Is(recordErr, fs.ErrNotExist) {
		return failed, recordErr
	}

	outcome, found, err := service.ports.RetainedOutcome(ctx, stateRoot, id)
	if err != nil {
		return failed, errors.Join(recordErr, err)
	}
	if !found {
		return failed, recordErr
	}
	if view == issueopsviewcontract.ViewGraph {
		return failed, fmt.Errorf("graph needs a live record; %s", issueopsdomain.RetiredCycleNote(outcome))
	}
	timeline := issueopsviewdomain.RetainedTimeline(outcome)
	return issueopsviewcontract.Result{
		OK: true, View: view, ID: outcome.ID, Source: issueopsviewcontract.SourceRetained,
		GeneratedAt: service.generatedAt(), Outcome: outcome.Outcome, RetainedAt: outcome.RetainedAt,
		Timeline: &timeline,
	}, nil
}

// graph builds the evidence graph of a live record. A failed `next` is not a
// failed view: the graph is still returned with every binding edge unknown and
// the error among the warnings.
func (service *Service) graph(ctx context.Context, stateRoot, cwd, id string, record issueopsviewcontract.Record) issueopsviewcontract.Result {
	result := issueopsviewcontract.Result{
		OK: true, View: issueopsviewcontract.ViewGraph, ID: id, Source: issueopsviewcontract.SourceRecord,
	}
	var judgment issueopsviewdomain.Judgment
	if next, err := service.ports.Next(ctx, stateRoot, cwd, id); err != nil {
		judgment.NextFailed = true
		result.Warnings = append(result.Warnings, fmt.Sprintf("next failed: %v", err))
	} else {
		judgment.Stage, judgment.Missing = next.Stage.Key, next.Missing
	}
	if record.Execution != nil {
		judgment.WorktreeMissing = !service.ports.WorktreePresent(ctx, record.Execution.Workspace.Root)
	}
	graph := issueopsviewdomain.EvidenceGraph(record, judgment)
	result.Graph = &graph
	result.GeneratedAt = service.generatedAt()
	return result
}

// errDependencies reports a Service built without the ports a view needs.
var errDependencies = errors.New("issueops view dependencies are required")

func (service *Service) generatedAt() string {
	return service.ports.Now().UTC().Format(time.RFC3339)
}
