package issueopscleanup

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
)

type ChildCleanupRecords interface {
	WithinLock(context.Context, string, func() error) error
	Load(string) (model.IssueOpsRecord, error)
	Save(model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

type ChildrenCloser struct {
	Records  ChildCleanupRecords
	Provider func(string) (port.IssueProvider, error)
	Now      func() time.Time
}

func (s ChildrenCloser) Close(ctx context.Context, id string, req model.IssueOpsCloseChildrenRequest) (model.IssueOpsCloseChildrenResult, error) {
	result := model.IssueOpsCloseChildrenResult{OK: false, ID: id}
	err := s.Records.WithinLock(ctx, id, func() error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		result, err = domain.PrepareChildCleanup(record, req)
		if err != nil {
			return err
		}
		basis, err := s.evidenceBasis(record, req)
		if err != nil {
			result.Missing = []string{"merge_evidence"}
			return err
		}
		result.EvidenceBasis = basis
		indices := make([]int, 0)
		for index, link := range record.IssueLinks {
			if link.Type == "child" {
				indices = append(indices, index)
			}
		}
		outcomes := s.closeConcurrently(record, indices, req)
		var changed []int
		var firstErr error
		for index, outcome := range outcomes {
			result.Children = append(result.Children, outcome.result)
			if outcome.err != nil && firstErr == nil {
				firstErr = outcome.err
			}
			if outcome.changed {
				changed = append(changed, indices[index])
			}
			if outcome.result.Closed {
				result.ClosedCount++
			}
		}
		if firstErr != nil {
			return firstErr
		}
		if req.Confirm && len(changed) > 0 {
			record = domain.ApplyChildCleanupReceipts(record, changed, s.Now().UTC().Format(time.RFC3339Nano))
			_, err = s.Records.Save(record)
			return err
		}
		return nil
	})
	return result, err
}

type closeChildOutcome struct {
	result  model.IssueOpsCloseChildResult
	changed bool
	err     error
}

func (s ChildrenCloser) closeConcurrently(record model.IssueOpsRecord, indices []int, req model.IssueOpsCloseChildrenRequest) []closeChildOutcome {
	outcomes := make([]closeChildOutcome, len(indices))
	limit := make(chan struct{}, 4)
	var workers sync.WaitGroup
	workers.Add(len(indices))
	for index, linkIndex := range indices {
		go func() {
			defer workers.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			result, changed, err := s.closeChild(record, record.IssueLinks[linkIndex], req)
			outcomes[index] = closeChildOutcome{result: result, changed: changed, err: err}
		}()
	}
	workers.Wait()
	return outcomes
}

func (s ChildrenCloser) evidenceBasis(record model.IssueOpsRecord, req model.IssueOpsCloseChildrenRequest) (string, error) {
	basis, err := domain.ChildCleanupEvidenceBasis(record, req.Merged)
	if err != nil || basis != "" {
		return basis, err
	}
	for _, link := range record.IssueLinks {
		if link.Type != "child" {
			continue
		}
		state, err := s.observeChildState(record, link)
		if err != nil {
			return "", fmt.Errorf("cannot close child tasks without merge evidence: %w", err)
		}
		if err := domain.ValidateClosedChildObservation(link.URL, state); err != nil {
			return "", err
		}
	}
	return domain.ChildCleanupAlreadyClosed, nil
}

func (s ChildrenCloser) observeChildState(record model.IssueOpsRecord, link model.IssueOpsIssueLink) (string, error) {
	providerName := remote.CleanupChildProvider(link.Provider, link.URL, record.IssueURL)
	if providerName == "" {
		return "", fmt.Errorf("cannot determine provider for child %s", link.URL)
	}
	prov, err := s.Provider(providerName)
	if err != nil {
		return "", err
	}
	result, err := prov.CloseChild(port.IssueProviderCloseChildRequest{Repo: record.Repo, ParentIssueURL: record.IssueURL, ChildURL: link.URL, Confirm: false})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.State), nil
}

func (s ChildrenCloser) closeChild(record model.IssueOpsRecord, link model.IssueOpsIssueLink, req model.IssueOpsCloseChildrenRequest) (model.IssueOpsCloseChildResult, bool, error) {
	providerName := remote.CleanupChildProvider(link.Provider, link.URL, record.IssueURL)
	if providerName == "" {
		return model.IssueOpsCloseChildResult{URL: link.URL, Error: "cannot determine provider"}, false, fmt.Errorf("cannot determine provider for child %s", link.URL)
	}
	prov, err := s.Provider(providerName)
	if err != nil {
		return model.IssueOpsCloseChildResult{URL: link.URL, Provider: providerName, Error: err.Error()}, false, err
	}
	providerResult, err := prov.CloseChild(port.IssueProviderCloseChildRequest{Repo: record.Repo, ParentIssueURL: record.IssueURL, ChildURL: link.URL, Confirm: req.Confirm})
	result := model.IssueOpsCloseChildResult{
		URL: firstChildValue(providerResult.ChildURL, link.URL), Provider: firstChildValue(providerResult.Provider, providerName),
		Closed: providerResult.Closed, AlreadyClosed: providerResult.AlreadyClosed, HierarchyVerified: providerResult.HierarchyVerified,
		State: providerResult.State, Preview: providerResult.Preview,
	}
	if err != nil {
		result.Error = err.Error()
		return result, false, err
	}
	// Keep the requested child identity in validation errors even when the provider
	// returns a canonical URL for presentation.
	evidence := result
	evidence.URL = link.URL
	if err := domain.ValidateChildCloseConfirmation(evidence, req.Confirm); err != nil {
		result.Error = err.Error()
		return result, false, err
	}
	return result, req.Confirm, nil
}

func firstChildValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
