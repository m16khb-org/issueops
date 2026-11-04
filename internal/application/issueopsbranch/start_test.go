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
		Read: func(string, string) (model.Record, error) {
			return model.Record{}, errors.New("not found")
		},
		Write: func(_ string, record model.Record) (model.Record, error) {
			writes++
			return record, nil
		},
		NewID: func(string, string) string { return "io-v1" },
	}

	got, err := startWithFake(store, model.StartRequest{Repo: ".", Branch: "69-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.ID != "io-v1" || got.Phase != model.PhaseProblem || writes != 1 {
		t.Fatalf("unexpected new v1 record: %+v writes=%d", got, writes)
	}
}

func TestStartReturnsExistingRecordWithoutRewrite(t *testing.T) {
	existing := model.Record{OK: true, SchemaVersion: 1, ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: model.PhasePlan}
	store := &startFake{
		Read: func(string, string) (model.Record, error) { return existing, nil },
		Write: func(string, model.Record) (model.Record, error) {
			t.Fatal("existing record must not be rewritten")
			return model.Record{}, nil
		},
		NewID: func(string, string) string { return existing.ID },
	}

	got, err := startWithFake(store, model.StartRequest{Repo: existing.Repo, Branch: existing.Branch})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != existing.ID || got.Phase != existing.Phase {
		t.Fatalf("existing record changed: %+v", got)
	}
}

func TestStartNewRefusesExistingIDWithoutRewrite(t *testing.T) {
	existing := model.Record{OK: true, SchemaVersion: 1, ID: "io-v1", Repo: "/repo", Phase: model.PhasePlan}
	store := &startFake{
		Read: func(string, string) (model.Record, error) { return existing, nil },
		Write: func(string, model.Record) (model.Record, error) {
			t.Fatal("an explicit new cycle must not overwrite an existing record")
			return model.Record{}, nil
		},
		NewID: func(string, string) string { return existing.ID },
	}

	if _, err := startWithFake(store, model.StartRequest{Repo: existing.Repo, New: true}); err == nil {
		t.Fatal("explicit new start must reject an id collision")
	}
}

func TestStartNewRefusesUnreadableIDWithoutRewrite(t *testing.T) {
	store := &startFake{
		Read: func(string, string) (model.Record, error) {
			return model.Record{}, errors.New("invalid state")
		},
		Write: func(string, model.Record) (model.Record, error) {
			t.Fatal("an explicit new cycle must not overwrite an unreadable record")
			return model.Record{}, nil
		},
		NewID: func(string, string) string { return "io-v1" },
	}

	if _, err := startWithFake(store, model.StartRequest{Repo: "/repo", New: true}); err == nil {
		t.Fatal("explicit new start must preserve an unreadable record")
	}
}

func TestStartCanonicalizesLinkedWorktreeRepoBeforeIDAndWrite(t *testing.T) {
	const (
		worktree = "/repo.worktrees/69-v1"
		source   = "/repo"
	)
	var idRepo string
	var written model.Record
	store := &startFake{
		Read: func(string, string) (model.Record, error) {
			return model.Record{}, errors.New("not found")
		},
		Write: func(_ string, record model.Record) (model.Record, error) {
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

	if _, err := startWithFake(store, model.StartRequest{Repo: worktree, Branch: "69-v1"}); err != nil {
		t.Fatal(err)
	}
	if idRepo != source || written.Repo != source {
		t.Fatalf("canonical repo mismatch: id repo=%q record repo=%q want=%q", idRepo, written.Repo, source)
	}
}

type startFake struct {
	Read          func(string, string) (model.Record, error)
	Write         func(string, model.Record) (model.Record, error)
	NewID         func(string, string) string
	NormalizeRepo func(string) string
	Independent   func(string) (string, error)
	Retired       map[string]bool
	DoneIDs       map[string]bool
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
func (f *startFake) Load(id string) (model.Record, error) {
	if !f.Locked {
		panic("read outside lock")
	}
	f.Events = append(f.Events, "read:"+id)
	return f.Read("state", id)
}
func (f *startFake) Save(_ context.Context, r model.Record) (model.Record, error) {
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
func (f *startFake) Done(id string) bool {
	f.Events = append(f.Events, "done?:"+id)
	return f.DoneIDs[id]
}
func (f *startFake) IndependentID(repo string) (string, error) {
	f.Events = append(f.Events, "independent:"+repo)
	if f.Independent != nil {
		return f.Independent(repo)
	}
	return f.NewID(repo, ""), nil
}
func startWithFake(f *startFake, req model.StartRequest) (model.Record, error) {
	return (Starter{Records: f, Retired: f, Identity: f, Now: func() time.Time { return time.Date(2026, 9, 28, 1, 2, 3, 4, time.UTC) }}).Start(context.Background(), req)
}
func TestStartSerializesCanonicalRecordAndFreezesIndependentID(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(fmtBool(fresh), func(t *testing.T) {
			f := &startFake{NormalizeRepo: func(string) string { return "/source" }, NewID: func(string, string) string { return "io-stable" }, Independent: func(string) (string, error) { return "io-fresh", nil }, Read: func(string, string) (model.Record, error) { return model.Record{}, fs.ErrNotExist }, Write: func(_ string, r model.Record) (model.Record, error) { return r, nil }}
			got, err := startWithFake(f, model.StartRequest{Repo: " /worktree ", New: fresh})
			if err != nil {
				t.Fatal(err)
			}
			id := "io-stable"
			want := []string{"normalize: /worktree ", "retired", "stable:/source::0", "done?:io-stable", "lock:io-stable", "normalize:/worktree", "read:io-stable", "write:io-stable", "unlock"}
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
		req     model.StartRequest
		setup   func(*startFake)
		want    string
		wrapped bool
	}{
		{"branchless precondition", model.StartRequest{New: true, Branch: "bad"}, func(f *startFake) { f.Independent = func(string) (string, error) { panic("must not generate ID") } }, "branchless", false},
		{"identity failure", model.StartRequest{Repo: "/repo", New: true}, func(f *startFake) { f.Independent = func(string) (string, error) { return "", cause } }, "unavailable", true},
		{"lock failure", model.StartRequest{Repo: "/repo"}, func(f *startFake) { f.LockError = cause }, "unavailable", true},
		{"empty repo precedes branch", model.StartRequest{Branch: "bad"}, func(*startFake) {}, "repo is required", false},
		{"canonical repo empty", model.StartRequest{Repo: "/repo", Branch: "bad"}, func(f *startFake) { f.NormalizeRepo = func(string) string { return "" } }, "repo is required", false},
		{"branch validation", model.StartRequest{Repo: "/repo", Branch: "bad"}, func(*startFake) {}, "issue number", false},
		{"unreadable new", model.StartRequest{Repo: "/repo", New: true}, func(f *startFake) {
			f.Read = func(string, string) (model.Record, error) { return model.Record{}, cause }
		}, "unreadable", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := &startFake{NewID: func(string, string) string { return "io-test" }, Read: func(string, string) (model.Record, error) { panic("unexpected read") }, Write: func(string, model.Record) (model.Record, error) { panic("unexpected write") }}
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
	existing := model.Record{OK: true, SchemaVersion: 1, ID: "io-stable", Repo: "/repo", Phase: model.PhasePlan, CreatedAt: "original", UpdatedAt: "unchanged", IssueURL: "https://github.com/acme/repo/issues/5"}
	f := &startFake{NewID: func(string, string) string { return existing.ID }, Read: func(string, string) (model.Record, error) { return existing, nil }, Write: func(string, model.Record) (model.Record, error) { panic("resume rewrote record") }}
	got, err := (Starter{Records: f, Retired: f, Identity: f, Now: func() time.Time { panic("resume read clock") }}).Start(context.Background(), model.StartRequest{Repo: "/repo"})
	if err != nil || !reflect.DeepEqual(got, existing) {
		t.Fatalf("record=%+v error=%v", got, err)
	}
}
func TestStartReturnsWriteFailureAndUnlocks(t *testing.T) {
	cause := errors.New("disk full")
	f := &startFake{NewID: func(string, string) string { return "io-test" }, Read: func(string, string) (model.Record, error) { return model.Record{}, fs.ErrNotExist }, Write: func(string, model.Record) (model.Record, error) { return model.Record{}, cause }}
	got, err := startWithFake(f, model.StartRequest{Repo: "/repo"})
	if !errors.Is(err, cause) || got.OK || f.Locked || f.Events[len(f.Events)-1] != "unlock" {
		t.Fatalf("record=%+v err=%v events=%v", got, err, f.Events)
	}
}

// Cleanup keeps an outcome row under a deleted record's ID; a later start in
// the same repo and branch must not reuse that ID, and repeating it resumes
// the same successor.
func TestStartSkipsIDsOfCleanedUpCycles(t *testing.T) {
	created := map[string]model.Record{}
	f := &startFake{
		NewID:   func(string, string) string { return "io-stable" },
		Retired: map[string]bool{"io-stable": true, "io-stable-1": true},
		Read: func(_ string, id string) (model.Record, error) {
			if r, ok := created[id]; ok {
				return r, nil
			}
			return model.Record{}, fs.ErrNotExist
		},
		Write: func(_ string, r model.Record) (model.Record, error) { created[r.ID] = r; return r, nil },
	}
	first, err := startWithFake(f, model.StartRequest{Repo: "/source"})
	if err != nil || first.ID != "io-stable-2" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	again, err := startWithFake(f, model.StartRequest{Repo: "/source"})
	if err != nil || again.ID != first.ID || len(created) != 1 {
		t.Fatalf("repeat start must resume %s: got %+v err=%v created=%d", first.ID, again, err, len(created))
	}
}

// A done cycle awaiting merge takes no new work: plain start moves to the next
// successor instead of returning the finished cycle (QA: a second task in the
// same checkout was attached to the done cycle).
func TestStartSkipsADoneCycle(t *testing.T) {
	var saved model.Record
	store := &startFake{
		Read:    func(string, string) (model.Record, error) { return model.Record{}, fs.ErrNotExist },
		Write:   func(_ string, r model.Record) (model.Record, error) { saved = r; return r, nil },
		NewID:   func(string, string) string { return "io-stable" },
		DoneIDs: map[string]bool{"io-stable": true},
	}
	got, err := startWithFake(store, model.StartRequest{Repo: "/repo"})
	if err != nil || got.ID != "io-stable-1" || saved.ID != "io-stable-1" || got.Phase != model.PhaseProblem {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

// Probe: `start --repo <plain dir>` created a cycle that could only fail at
// branch prepare; a repo outside git is refused before any ID or write.
func TestStartRefusesARepoOutsideGit(t *testing.T) {
	f := &startFake{NewID: func(string, string) string { panic("refused start derived an id") }, Write: func(string, model.Record) (model.Record, error) { panic("refused start wrote") }}
	starter := Starter{Records: f, Retired: f, Identity: f, IsGitRepo: func(string) bool { return false }, Now: time.Now}
	if _, err := starter.Start(context.Background(), model.StartRequest{Repo: "/plain"}); err == nil || !strings.Contains(err.Error(), "repo /plain is not a git repository") {
		t.Fatalf("err = %v", err)
	}
}
