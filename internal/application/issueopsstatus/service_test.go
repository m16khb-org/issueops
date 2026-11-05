package issueopsstatus

import (
	"context"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestServiceReadsAndProjectsStatusWithoutPersisting(t *testing.T) {
	repository := &fakeRepository{
		record: issueopscontract.Record{
			OK:    true,
			ID:    "io-status01",
			Phase: issueopscontract.PhasePlan,
		},
	}
	projector := &fakeProjector{}
	service := NewService(repository, projector)

	record, err := service.Status(context.Background(), "/state", "io-status01")
	if err != nil {
		t.Fatal(err)
	}
	if repository.reads != 1 || projector.projections != 1 {
		t.Fatalf("status must read and project once: reads=%d projections=%d", repository.reads, projector.projections)
	}
	if record.SourceMisdirectWarnings != 1 {
		t.Fatalf("projected record not returned: %+v", record)
	}
}

type fakeRepository struct {
	record issueopscontract.Record
	reads  int
}

func (repository *fakeRepository) Read(context.Context, string, string) (issueopscontract.Record, error) {
	repository.reads++
	return repository.record, nil
}

type fakeProjector struct {
	projections int
}

func (projector *fakeProjector) Project(record issueopscontract.Record) issueopscontract.Record {
	projector.projections++
	record.SourceMisdirectWarnings++
	return record
}
