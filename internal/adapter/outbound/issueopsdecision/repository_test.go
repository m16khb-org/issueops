package issueopsdecision

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestRepositoryPersistsDecisionMutationWithinSpan(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-decision01"
	data, err := json.Marshal(issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id,
		Phase:         issueopscontract.PhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put("issueops_v1", id, data); err != nil {
		t.Fatal(err)
	}

	record, err := (Repository{}).Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.Record) (issueopscontract.Record, error) {
			record.Decisions = append(record.Decisions, issueopscontract.Decision{
				Title: "Boundary",
			})
			return record, nil
		},
	)
	if err != nil || len(record.Decisions) != 1 {
		t.Fatalf("update returned %+v, %v", record, err)
	}
}
