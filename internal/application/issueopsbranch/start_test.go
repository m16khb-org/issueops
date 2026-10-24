package issueopsbranch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

func TestStartCreatesSchemaOneRecord(t *testing.T) {
	writes := 0
	store := &startFake{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{}, errors.New("not found")
		},
		Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return record, nil
		},
		NewID: func(string, string) string { return "io-v1" },
	}

	got, err := startWithFake(store, model.IssueOpsStartRequest{Repo: ".", Branch: "69-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.ID != "io-v1" || got.Phase != model.IssueOpsPhaseProblem || writes != 1 {
		t.Fatalf("unexpected new v1 record: %+v writes=%d", got, writes)
	}
}

func TestStartReturnsExistingRecordWithoutRewrite(t *testing.T) {
	existing := model.IssueOpsRecord{OK: true, SchemaVersion: 1, ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: model.IssueOpsPhasePlan}
	store := &startFake{
		Read: func(string, string) (model.IssueOpsRecord, error) { return existing, nil },
		Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("existing record must not be rewritten")
			return model.IssueOpsRecord{}, nil
		},
		NewID: func(string, string) string { return existing.ID },
	}

	got, err := startWithFake(store, model.IssueOpsStartRequest{Repo: existing.Repo, Branch: existing.Branch})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != existing.ID || got.Phase != existing.Phase {
		t.Fatalf("existing record changed: %+v", got)
	}
}

func TestStartNewRefusesExistingIDWithoutRewrite(t *testing.T) {
	existing := model.IssueOpsRecord{OK: true, SchemaVersion: 1, ID: "io-v1", Repo: "/repo", Phase: model.IssueOpsPhasePlan}
	store := &startFake{
		Read: func(string, string) (model.IssueOpsRecord, error) { return existing, nil },
		Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("an explicit new cycle must not overwrite an existing record")
			return model.IssueOpsRecord{}, nil
		},
		NewID: func(string, string) string { return existing.ID },
	}

	if _, err := startWithFake(store, model.IssueOpsStartRequest{Repo: existing.Repo, New: true}); err == nil {
		t.Fatal("explicit new start must reject an id collision")
	}
}

func TestStartNewRefusesUnreadableIDWithoutRewrite(t *testing.T) {
	store := &startFake{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{}, errors.New("invalid state")
		},
		Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("an explicit new cycle must not overwrite an unreadable record")
			return model.IssueOpsRecord{}, nil
		},
		NewID: func(string, string) string { return "io-v1" },
	}

	if _, err := startWithFake(store, model.IssueOpsStartRequest{Repo: "/repo", New: true}); err == nil {
		t.Fatal("explicit new start must preserve an unreadable record")
	}
}

func TestStartCanonicalizesLinkedWorktreeRepoBeforeIDAndWrite(t *testing.T) {
	const (
		worktree = "/repo.worktrees/69-v1"
		source   = "/repo"
	)
	var idRepo string
	var written model.IssueOpsRecord
	store := &startFake{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{}, errors.New("not found")
		},
		Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			written = record
			return record, nil
		},
		NewID: func(repo, _ string) string {
			idRepo = repo
			return "io-v1"
		},

		NormalizeRepo: func(repo string) string {
			if repo != worktree {
				t.Fatalf("NormalizeRepo input=%q, want %q", repo, worktree)
			}
			return source
		},
	}

	if _, err := startWithFake(store, model.IssueOpsStartRequest{Repo: worktree, Branch: "69-v1"}); err != nil {
		t.Fatal(err)
	}
	if idRepo != source || written.Repo != source {
		t.Fatalf("canonical repo mismatch: id repo=%q record repo=%q want=%q", idRepo, written.Repo, source)
	}
}

type startFake struct {
	Read          func(string, string) (model.IssueOpsRecord, error)
	Write         func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	NewID         func(string, string) string
	NormalizeRepo func(string) string
	Independent   func(string) (string, error)
	Retired       map[string]bool
	LockError     error
	Events        []string
	Locked        bool
}

func (f *startFake) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	f.Events = append(f.Events, "lock:"+id)
	if f.LockError != nil {
		return f.LockError
	}
	f.Locked = true
	defer func() { f.Locked = false; f.Events = append(f.Events, "unlock") }()
	return fn(ctx)
}
func (f *startFake) Load(id string) (model.IssueOpsRecord, error) {
	if !f.Locked {
		panic("read outside lock")
	}
	f.Events = append(f.Events, "read:"+id)
	return f.Read("state", id)
}
func (f *startFake) Save(_ context.Context, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if !f.Locked {
		panic("write outside lock")
	}
	f.Events = append(f.Events, "write:"+r.ID)
	return f.Write("state", r)
}
func (f *startFake) CanonicalRepo(repo string) string {
	f.Events = append(f.Events, "normalize:"+repo)
	if f.NormalizeRepo != nil {
		return f.NormalizeRepo(repo)
	}
	return repo
}
func (f *startFake) SuccessorID(repo, branch string, n int) string {
	f.Events = append(f.Events, fmt.Sprintf("stable:%s:%s:%d", repo, branch, n))
	id := f.NewID(repo, branch)
	if n > 0 {
		id += fmt.Sprintf("-%d", n)
	}
	return id
}
func (f *startFake) RetiredIDs() (map[string]bool, error) {
	f.Events = append(f.Events, "retired")
	return f.Retired, nil
}
func (f *startFake) IndependentID(repo string) (string, error) {
	f.Events = append(f.Events, "independent:"+repo)
	if f.Independent != nil {
		return f.Independent(repo)
	}
	return f.NewID(repo, ""), nil
}
func startWithFake(f *startFake, req model.IssueOpsStartRequest) (model.IssueOpsRecord, error) {
	return (Starter{Records: f, Retired: f, Identity: f, Now: func() time.Time { return time.Date(2026, 9, 28, 1, 2, 3, 4, time.UTC) }}).Start(context.Background(), req)
}
func TestStartSerializesCanonicalRecordAndFreezesIndependentID(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(fmtBool(fresh), func(t *testing.T) {
			f := &startFake{NormalizeRepo: func(string) string { return "/source" }, NewID: func(string, string) string { return "io-stable" }, Independent: func(string) (string, error) { return "io-fresh", nil }, Read: func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, fs.ErrNotExist }, Write: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) { return r, nil }}
			got, err := startWithFake(f, model.IssueOpsStartRequest{Repo: " /worktree ", New: fresh})
			if err != nil {
				t.Fatal(err)
			}
			id := "io-stable"
			want := []string{"normalize: /worktree ", "retired", "stable:/source::0", "lock:io-stable", "normalize:/worktree", "read:io-stable", "write:io-stable", "unlock"}
			if fresh {
				id = "io-fresh"
				want = []string{"normalize: /worktree ", "independent:/source", "lock:io-fresh", "normalize:/worktree", "read:io-fresh", "write:io-fresh", "unlock"}
			}
			if !reflect.DeepEqual(f.Events, want) {
				t.Fatalf("events=%v want=%v", f.Events, want)
			}
			if got.ID != id || got.Repo != "/source" || got.CreatedAt != "2026-09-28T01:02:03.000000004Z" || got.UpdatedAt != got.CreatedAt || got.Feedback == nil {
				t.Fatalf("record=%+v", got)
			}
		})
	}
}
func fmtBool(value bool) string {
	if value {
		return "new"
	}
	return "resume"
}
func TestStartFailureOrderAndNoWrite(t *testing.T) {
	cause := errors.New("unavailable")
	cases := []struct {
		name    string
		req     model.IssueOpsStartRequest
		setup   func(*startFake)
		want    string
		wrapped bool
	}{
		{"branchless precondition", model.IssueOpsStartRequest{New: true, Branch: "bad"}, func(f *startFake) { f.Independent = func(string) (string, error) { panic("must not generate ID") } }, "branchless", false},
		{"identity failure", model.IssueOpsStartRequest{Repo: "/repo", New: true}, func(f *startFake) { f.Independent = func(string) (string, error) { return "", cause } }, "unavailable", true},
		{"lock failure", model.IssueOpsStartRequest{Repo: "/repo"}, func(f *startFake) { f.LockError = cause }, "unavailable", true},
		{"empty repo precedes branch", model.IssueOpsStartRequest{Branch: "bad"}, func(*startFake) {}, "repo is required", false},
		{"canonical repo empty", model.IssueOpsStartRequest{Repo: "/repo", Branch: "bad"}, func(f *startFake) { f.NormalizeRepo = func(string) string { return "" } }, "repo is required", false},
		{"branch validation", model.IssueOpsStartRequest{Repo: "/repo", Branch: "bad"}, func(*startFake) {}, "issue number", false},
		{"unreadable new", model.IssueOpsStartRequest{Repo: "/repo", New: true}, func(f *startFake) {
			f.Read = func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, cause }
		}, "unreadable", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := &startFake{NewID: func(string, string) string { return "io-test" }, Read: func(string, string) (model.IssueOpsRecord, error) { panic("unexpected read") }, Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) { panic("unexpected write") }}
			tt.setup(f)
			got, err := startWithFake(f, tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) || got.OK {
				t.Fatalf("record=%+v error=%v", got, err)
			}
			if tt.wrapped && !errors.Is(err, cause) {
				t.Fatalf("lost cause: %v", err)
			}
		})
	}
}

func TestStartResumePreservesWholeRecordAndDoesNotReadClock(t *testing.T) {
	existing := model.IssueOpsRecord{OK: true, SchemaVersion: 1, ID: "io-stable", Repo: "/repo", Phase: model.IssueOpsPhasePlan, CreatedAt: "original", UpdatedAt: "unchanged", IssueURL: "https://github.com/acme/repo/issues/5"}
	f := &startFake{NewID: func(string, string) string { return existing.ID }, Read: func(string, string) (model.IssueOpsRecord, error) { return existing, nil }, Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) { panic("resume rewrote record") }}
	got, err := (Starter{Records: f, Retired: f, Identity: f, Now: func() time.Time { panic("resume read clock") }}).Start(context.Background(), model.IssueOpsStartRequest{Repo: "/repo"})
	if err != nil || !reflect.DeepEqual(got, existing) {
		t.Fatalf("record=%+v error=%v", got, err)
	}
}
func TestStartReturnsWriteFailureAndUnlocks(t *testing.T) {
	cause := errors.New("disk full")
	f := &startFake{NewID: func(string, string) string { return "io-test" }, Read: func(string, string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, fs.ErrNotExist }, Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, cause }}
	got, err := startWithFake(f, model.IssueOpsStartRequest{Repo: "/repo"})
	if !errors.Is(err, cause) || got.OK || f.Locked || f.Events[len(f.Events)-1] != "unlock" {
		t.Fatalf("record=%+v err=%v events=%v", got, err, f.Events)
	}
}

// Cleanup keeps an outcome row under a deleted record's ID; a later start in
// the same repo and branch must not reuse that ID, and repeating it resumes
// the same successor.
func TestStartSkipsIDsOfCleanedUpCycles(t *testing.T) {
	created := map[string]model.IssueOpsRecord{}
	f := &startFake{
		NewID:   func(string, string) string { return "io-stable" },
		Retired: map[string]bool{"io-stable": true, "io-stable-1": true},
		Read: func(_ string, id string) (model.IssueOpsRecord, error) {
			if r, ok := created[id]; ok {
				return r, nil
			}
			return model.IssueOpsRecord{}, fs.ErrNotExist
		},
		Write: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) { created[r.ID] = r; return r, nil },
	}
	first, err := startWithFake(f, model.IssueOpsStartRequest{Repo: "/source"})
	if err != nil || first.ID != "io-stable-2" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	again, err := startWithFake(f, model.IssueOpsStartRequest{Repo: "/source"})
	if err != nil || again.ID != first.ID || len(created) != 1 {
		t.Fatalf("repeat start must resume %s: got %+v err=%v created=%d", first.ID, again, err, len(created))
	}
}
