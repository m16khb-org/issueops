package issueops

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestChildStartRollsBackParentAndChildWhenSecondWriteFails(t *testing.T) {
	root := t.TempDir()
	parent := createDelegationReadyParentForTest(t, root)
	req := model.IssueOpsChildStartRequest{ParentID: parent.ID, Branch: "124-atomic-child", Title: "atomic", TaskScope: "atomic parent and child", AcceptanceCriteria: []string{"no partial graph"}}
	id := CycleStartIdentity{}.StableID(parent.Repo, req.Branch)
	db, err := sql.Open("sqlite", filepath.Join(root, "issueops.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("CREATE TRIGGER fail_child BEFORE INSERT ON records WHEN NEW.id = '%s' BEGIN SELECT RAISE(ABORT, 'injected child write failure'); END", id)); err != nil {
		t.Fatal(err)
	}

	actor := issueOpsActorForTest(parent.WorktreePath)
	if _, err := childStarterForTest(root).Start(context.Background(), req, &actor); err == nil || !strings.Contains(err.Error(), "injected child write failure") {
		t.Fatalf("missing injected failure: %v", err)
	}
	after, err := ReadIssueOps(root, parent.ID)
	if err != nil || !reflect.DeepEqual(after, parent) {
		t.Fatalf("failed child write changed parent: %v", err)
	}
	if _, err := ReadIssueOps(root, id); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("partial child remains: %v", err)
	}
	if _, err := db.Exec("DROP TRIGGER fail_child"); err != nil {
		t.Fatal(err)
	}
	started, err := childStarterForTest(root).Start(context.Background(), req, &actor)
	if err != nil {
		t.Fatal(err)
	}
	after, err = ReadIssueOps(root, parent.ID)
	if err != nil || len(after.ChildCycles) != 1 || after.ChildCycles[0].CycleID != started.Child.ID {
		t.Fatalf("retry did not commit graph: %+v %v", after.ChildCycles, err)
	}
	if started.Child.DevilsAdvocateReview == nil || started.Child.DevilsAdvocateReview.ReviewerPattern != "delegated-parent-review" || !started.Child.DevilsAdvocateReview.Waived {
		t.Fatalf("inherited review lost: %+v", started.Child.DevilsAdvocateReview)
	}
}

// These cases protect the second record now that child creation uses the parent's
// span instead of entering a separate child lock.
func TestChildStartRefusalPreservesBothRows(t *testing.T) {
	for _, scenario := range []string{"parent fence", "child fence", "corrupt child", "wrong actor"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			parent := createDelegationReadyParentForTest(t, root)
			req := model.IssueOpsChildStartRequest{ParentID: parent.ID, Branch: "124-refused-child", TaskScope: "preserve the graph", AcceptanceCriteria: []string{"no writes on refusal"}}
			actor := issueOpsActorForTest(parent.WorktreePath)
			started, err := childStarterForTest(root).Start(context.Background(), req, &actor)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			switch scenario {
			case "parent fence", "child fence":
				id := parent.ID
				if scenario == "child fence" {
					id = started.Child.ID
				}
				record, err := ReadIssueOps(root, id)
				if err != nil {
					t.Fatal(err)
				}
				record.CleanupAbandonFailure = &model.IssueOpsCleanupAbandonFailure{Step: "applying", Fingerprint: strings.Repeat("a", 64), At: "2026-08-04T00:00:00Z"}
				if _, err := writeIssueOps(root, record); err != nil {
					t.Fatal(err)
				}
				want = "cleanup abandon apply is in progress"
			case "corrupt child":
				if err := db.Put(issueOpsBucket, started.Child.ID, []byte("{broken")); err != nil {
					t.Fatal(err)
				}
			case "wrong actor":
				actor.CWD = t.TempDir()
			}
			before := map[string][]byte{}
			for _, id := range []string{parent.ID, started.Child.ID} {
				raw, found, err := db.Get(issueOpsBucket, id)
				if err != nil || !found {
					t.Fatalf("snapshot %s: found=%v err=%v", id, found, err)
				}
				before[id] = raw
			}
			result, err := childStarterForTest(root).Start(context.Background(), req, &actor)
			if err == nil || result.OK || (want != "" && !strings.Contains(err.Error(), want)) {
				t.Fatalf("refusal: result=%+v err=%v", result, err)
			}
			for id, raw := range before {
				after, found, err := db.Get(issueOpsBucket, id)
				if err != nil || !found || !bytes.Equal(after, raw) {
					t.Fatalf("refusal changed %s: %v", id, err)
				}
			}
		})
	}
}
