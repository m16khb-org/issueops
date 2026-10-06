package issueopsrecord

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestStoreScanEachPropagatesVisitorNotExist(t *testing.T) {
	root := t.TempDir()
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-valid", Repo: "/repo", Phase: issueopscontract.IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(bucket, "io-valid", data); err != nil {
		t.Fatal(err)
	}
	visited := 0
	diagnostics, err := (Store{}).ScanEach(context.Background(), root, func(issueopscontract.IssueOpsRecord) error {
		visited++
		return fs.ErrNotExist
	})
	if !errors.Is(err, fs.ErrNotExist) || visited != 1 || diagnostics != nil {
		t.Fatalf("visitor error swallowed: visited=%d diagnostics=%v err=%v", visited, diagnostics, err)
	}
}
