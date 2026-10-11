package nativeactivation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/port"
	activationport "issueops/internal/port/nativeactivation"
)

type fixture struct {
	root, issueOpsRoot, binDir, target string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{root: root, issueOpsRoot: filepath.Join(root, "issueops")}
	f.binDir = filepath.Join(f.issueOpsRoot, "bin")
	f.target = filepath.Join(f.binDir, "issueops")
	if err := os.MkdirAll(f.binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.target, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) begin() activationport.BeginRequest {
	return activationport.BeginRequest{StateRoot: f.root, IssueOpsRoot: f.issueOpsRoot, TargetBinary: f.target}
}

func (f fixture) seal(id string) activationport.SealRequest {
	return activationport.SealRequest{StateRoot: f.root, IssueOpsRoot: f.issueOpsRoot, TargetBinary: f.target, TransitionID: id, CatalogSHA256: "catalog"}
}

func (f fixture) abort(id string) activationport.AbortRequest {
	return activationport.AbortRequest{StateRoot: f.root, IssueOpsRoot: f.issueOpsRoot, TargetBinary: f.target, TransitionID: id}
}

func (f fixture) put(t *testing.T, id string, data string) {
	t.Helper()
	db, err := sqlstore.Open(filepath.Join(f.root, storeDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(storeBucket, id, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) get(t *testing.T, id string) (string, bool) {
	t.Helper()
	db, err := sqlstore.Open(filepath.Join(f.root, storeDirectory))
	if err != nil {
		t.Fatal(err)
	}
	data, ok, err := db.Get(storeBucket, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), ok
}

func wantError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

const validID = "00112233445566778899aabbccddeeff"

func TestValidatePathsRejectsEachNonCanonicalPathWithItsOwnMessage(t *testing.T) {
	f := newFixture(t)
	const pathsErr = "native activation paths must be absolute and canonical"
	for name, tc := range map[string]struct {
		state, issueOps, target, want string
	}{
		"empty state root":        {"", f.issueOpsRoot, f.target, pathsErr},
		"relative issueops root":  {f.root, "issueops", f.target, pathsErr},
		"unclean target":          {f.root, f.issueOpsRoot, f.binDir + "/../bin/issueops", pathsErr},
		"trailing slash state":    {f.root + "/", f.issueOpsRoot, f.target, pathsErr},
		"missing harness root":    {f.root, filepath.Join(f.root, "absent"), filepath.Join(f.root, "absent", "bin", "issueops"), "native activation harness root must be a physical directory"},
		"harness root is a file":  {f.root, f.target, filepath.Join(f.target, "bin", "issueops"), "native activation harness root must be a physical directory"},
		"target outside bin":      {f.root, f.issueOpsRoot, filepath.Join(f.issueOpsRoot, "issueops"), "native activation target must be the canonical harness binary"},
		"target in other harness": {f.root, f.issueOpsRoot, filepath.Join(f.root, "bin", "issueops"), "native activation target must be the canonical harness binary"},
	} {
		t.Run(name, func(t *testing.T) {
			wantError(t, validatePaths(tc.state, tc.issueOps, tc.target), tc.want)
		})
	}
	if err := validatePaths(f.root, f.issueOpsRoot, f.target); err != nil {
		t.Fatalf("canonical paths rejected: %v", err)
	}
	// A not-yet-installed target is allowed; only a symlink is refused.
	if err := os.Remove(f.target); err != nil {
		t.Fatal(err)
	}
	if err := validatePaths(f.root, f.issueOpsRoot, f.target); err != nil {
		t.Fatalf("absent target rejected: %v", err)
	}
}

func TestValidatePathsRejectsSymlinkedHarnessRootAndTarget(t *testing.T) {
	f := newFixture(t)
	link := filepath.Join(f.root, "harness-link")
	if err := os.Symlink(f.issueOpsRoot, link); err != nil {
		t.Fatal(err)
	}
	wantError(t, validatePaths(f.root, link, filepath.Join(link, "bin", "issueops")), "native activation harness root must be a physical directory")

	elsewhere := filepath.Join(f.root, "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, f.target); err != nil {
		t.Fatal(err)
	}
	wantError(t, validatePaths(f.root, f.issueOpsRoot, f.target), "native activation target must not be a symbolic link")
}

func TestBackendRejectsMisconfiguredDependencies(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	noExecutable := NewBackend(func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	noExecutable.executable = nil
	_, err := noExecutable.Begin(ctx, f.begin())
	wantError(t, err, "inspect native activation candidate: native activation executable inspector is unavailable")

	failing := testBackend(f.target)
	failing.executable = func() (string, error) { return "", errors.New("no exe") }
	_, err = failing.Begin(ctx, f.begin())
	wantError(t, err, "inspect native activation candidate: no exe")
	_, err = failing.Seal(ctx, f.seal(validID))
	wantError(t, err, "inspect native activation binary: no exe")
	_, err = failing.Abort(ctx, f.abort(validID))
	wantError(t, err, "inspect native activation abort candidate: no exe")

	noClock := testBackend(f.target)
	noClock.now = nil
	_, err = noClock.Begin(ctx, f.begin())
	wantError(t, err, "native activation clock is unavailable")

	noTransition := testBackend(f.target)
	noTransition.transition = nil
	_, err = noTransition.Begin(ctx, f.begin())
	wantError(t, err, "native activation transition ID generator is unavailable")

	for name, generate := range map[string]func() (string, error){
		"generator error":   func() (string, error) { return validID, errors.New("entropy") },
		"short id":          func() (string, error) { return "abcd", nil },
		"uppercase id":      func() (string, error) { return strings.ToUpper(validID), nil },
		"non-hex id":        func() (string, error) { return strings.Repeat("z", 32), nil },
		"empty id":          func() (string, error) { return "", nil },
		"too long hex id":   func() (string, error) { return validID + "00", nil },
		"odd length hex id": func() (string, error) { return validID[:31], nil },
	} {
		t.Run(name, func(t *testing.T) {
			backend := testBackend(f.target)
			backend.transition = generate
			_, err := backend.Begin(ctx, f.begin())
			wantError(t, err, "generate native activation transition ID")
		})
	}
	if _, ok := f.get(t, pendingID); ok {
		t.Fatal("rejected Begin persisted a pending record")
	}

	noFactory := NewBackend(nil)
	noFactory.executable = func() (string, error) { return f.target, nil }
	_, err = noFactory.Begin(ctx, f.begin())
	wantError(t, err, "native activation store factory is unavailable")

	nilStore := NewBackend(func(string) (port.TransactionalRecordStore, error) { return nil, nil })
	nilStore.executable = func() (string, error) { return f.target, nil }
	_, err = nilStore.Seal(ctx, f.seal(validID))
	wantError(t, err, "native activation store factory returned nil")
	_, err = nilStore.Abort(ctx, f.abort(validID))
	wantError(t, err, "native activation store factory returned nil")

	openFails := NewBackend(func(string) (port.TransactionalRecordStore, error) { return nil, errors.New("disk gone") })
	openFails.executable = func() (string, error) { return f.target, nil }
	_, err = openFails.Begin(ctx, f.begin())
	wantError(t, err, "disk gone")
}

func TestBeginRejectsCandidatesOutsideTargetOrSameDirectoryStage(t *testing.T) {
	f := newFixture(t)
	otherDir := filepath.Join(f.root, "other")
	if err := os.MkdirAll(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(otherDir, ".issueops.activate-x")
	unrelated := filepath.Join(f.binDir, "random-binary")
	for _, path := range []string{foreign, unrelated} {
		if err := os.WriteFile(path, []byte("x"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, candidate := range map[string]string{"stage in another directory": foreign, "unrelated same-directory binary": unrelated} {
		t.Run(name, func(t *testing.T) {
			_, err := testBackend(candidate).Begin(context.Background(), f.begin())
			wantError(t, err, "native activation candidate must be the canonical target or a same-directory staged binary")
		})
	}
	if _, ok := f.get(t, pendingID); ok {
		t.Fatal("rejected candidate persisted a pending record")
	}
}

func TestBeginPersistsPendingResultAndReplacesAnyPriorPending(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	fixed := time.Date(2026, 10, 12, 1, 2, 3, 4, time.FixedZone("x", 9*3600))
	backend.now = func() time.Time { return fixed }
	backend.transition = func() (string, error) { return validID, nil }

	result, err := backend.Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pending || result.UpdatedAt != "2026-10-11T16:02:03.000000004Z" || result.TransitionID != validID ||
		result.StateRoot != f.root || result.IssueOpsRoot != f.issueOpsRoot || result.TargetBinary != f.target {
		t.Fatalf("result = %+v", result)
	}
	stored, ok := f.get(t, pendingID)
	if !ok || !strings.Contains(stored, `"transition_id":"`+validID+`"`) || !strings.Contains(stored, result.BinarySHA256) {
		t.Fatalf("stored pending = %q ok=%v", stored, ok)
	}

	second := "ffeeddccbbaa99887766554433221100"
	backend.transition = func() (string, error) { return second, nil }
	if _, err := backend.Begin(context.Background(), f.begin()); err != nil {
		t.Fatal(err)
	}
	// The earlier transition must no longer be sealable.
	_, err = backend.Seal(context.Background(), f.seal(validID))
	wantError(t, err, "invalid native activation state")
	if _, err := backend.Seal(context.Background(), f.seal(second)); err != nil {
		t.Fatalf("latest transition not sealable: %v", err)
	}
}

func TestSealAndAbortRejectMalformedTransitionIDsBeforeReadingState(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	for _, id := range []string{"", "xyz", strings.ToUpper(validID), validID + "0"} {
		_, err := backend.Seal(context.Background(), f.seal(id))
		wantError(t, err, "native activation transition ID is invalid")
		_, err = backend.Abort(context.Background(), f.abort(id))
		wantError(t, err, "native activation transition ID is invalid")
	}
	if _, err := os.Stat(filepath.Join(f.root, storeDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected requests created the store: %v", err)
	}
}

func TestSealAndAbortRejectInvalidPathsBeforeInspectingBinary(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	backend.executable = func() (string, error) {
		t.Error("binary inspected before path validation")
		return f.target, nil
	}
	request := f.seal(validID)
	request.TargetBinary = filepath.Join(f.issueOpsRoot, "issueops")
	_, err := backend.Seal(context.Background(), request)
	wantError(t, err, "native activation target must be the canonical harness binary")
	abort := f.abort(validID)
	abort.IssueOpsRoot = "relative"
	_, err = backend.Abort(context.Background(), abort)
	wantError(t, err, "native activation paths must be absolute and canonical")
}

func TestSealMustRunFromCanonicalTargetNotStagedBinary(t *testing.T) {
	f := newFixture(t)
	stage := filepath.Join(f.binDir, ".issueops.activate-seal")
	if err := os.WriteFile(stage, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	begin, err := testBackend(stage).Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	_, err = testBackend(stage).Seal(context.Background(), f.seal(begin.TransitionID))
	wantError(t, err, "native activation seal must run from the canonical target binary")
	if _, ok := f.get(t, pendingID); !ok {
		t.Fatal("refused seal consumed the pending record")
	}
}

func TestSealWithoutPendingOrReceiptReportsMissingPending(t *testing.T) {
	f := newFixture(t)
	_, err := testBackend(f.target).Seal(context.Background(), f.seal(validID))
	wantError(t, err, "native activation pending record is missing")
}

func TestSealReplayRequiresReceiptToMatchEveryRequestedFact(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	begin, err := backend.Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	request := f.seal(begin.TransitionID)
	request.Evidence = []activationport.Evidence{{Host: "codex", Surface: "mcp", Path: "/p"}}
	sealed, err := backend.Seal(context.Background(), request)
	if err != nil || !sealed.Sealed {
		t.Fatalf("seal = %+v err=%v", sealed, err)
	}
	if _, ok := f.get(t, pendingID); ok {
		t.Fatal("seal left the pending record behind")
	}

	for name, mutate := range map[string]func(*activationport.SealRequest){
		"different catalog":  func(r *activationport.SealRequest) { r.CatalogSHA256 = "other" },
		"different evidence": func(r *activationport.SealRequest) { r.Evidence = []activationport.Evidence{{Host: "claude"}} },
		"missing evidence":   func(r *activationport.SealRequest) { r.Evidence = nil },
		"different id":       func(r *activationport.SealRequest) { r.TransitionID = "ffeeddccbbaa99887766554433221100" },
	} {
		t.Run(name, func(t *testing.T) {
			replay := request
			mutate(&replay)
			result, err := backend.Seal(context.Background(), replay)
			wantError(t, err, "invalid native activation state")
			if result.Sealed {
				t.Fatalf("mismatched replay reported sealed: %+v", result)
			}
		})
	}
	replayed, err := backend.Seal(context.Background(), request)
	if err != nil || replayed != sealed {
		t.Fatalf("exact replay = %+v err=%v", replayed, err)
	}
}

// The strict-decoding cases start from a genuine stored record, so a lenient
// decoder would accept them: only DisallowUnknownFields and the trailing-data
// check can reject them.
func TestSealAndAbortRejectStrictDecodingViolationsInRealRecords(t *testing.T) {
	corrupt := map[string]func(string) string{
		"unknown field": func(s string) string { return strings.TrimSuffix(s, "}") + `,"surprise":true}` },
		"trailing data": func(s string) string { return s + ` {}` },
	}
	for name, apply := range corrupt {
		t.Run("pending "+name, func(t *testing.T) {
			f := newFixture(t)
			backend := testBackend(f.target)
			begin, err := backend.Begin(context.Background(), f.begin())
			if err != nil {
				t.Fatal(err)
			}
			stored, _ := f.get(t, pendingID)
			f.put(t, pendingID, apply(stored))
			_, err = backend.Seal(context.Background(), f.seal(begin.TransitionID))
			wantError(t, err, "invalid native activation state")
			_, err = backend.Abort(context.Background(), f.abort(begin.TransitionID))
			wantError(t, err, "invalid native activation abort state")
			if _, ok := f.get(t, pendingID); !ok {
				t.Fatal("rejected abort deleted the pending record")
			}
		})
		t.Run("receipt "+name, func(t *testing.T) {
			f := newFixture(t)
			backend := testBackend(f.target)
			begin, err := backend.Begin(context.Background(), f.begin())
			if err != nil {
				t.Fatal(err)
			}
			request := f.seal(begin.TransitionID)
			if _, err := backend.Seal(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			stored, _ := f.get(t, receiptID)
			f.put(t, receiptID, apply(stored))
			_, err = backend.Seal(context.Background(), request)
			wantError(t, err, "invalid native activation state")
		})
	}
}

func TestSealAndAbortRejectUndecodableStoredRecords(t *testing.T) {
	for name, raw := range map[string]string{"not json": `{`, "wrong record shape": `[]`} {
		t.Run("pending "+name, func(t *testing.T) {
			f := newFixture(t)
			f.put(t, pendingID, raw)
			_, err := testBackend(f.target).Seal(context.Background(), f.seal(validID))
			wantError(t, err, "invalid native activation state")
			_, err = testBackend(f.target).Abort(context.Background(), f.abort(validID))
			wantError(t, err, "invalid native activation abort state")
			if _, ok := f.get(t, pendingID); !ok {
				t.Fatal("rejected abort deleted the pending record")
			}
		})
		t.Run("receipt "+name, func(t *testing.T) {
			f := newFixture(t)
			f.put(t, receiptID, raw)
			_, err := testBackend(f.target).Seal(context.Background(), f.seal(validID))
			wantError(t, err, "invalid native activation state")
		})
	}
}

func TestSealRefusesPendingRecordFromAnotherStateRootOrWithoutStartTime(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	begin, err := backend.Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := f.get(t, pendingID)
	for name, tampered := range map[string]string{
		"foreign state root": strings.Replace(stored, `"state_root":"`+f.root+`"`, `"state_root":"/elsewhere"`, 1),
		"missing start time": stored[:strings.Index(stored, `"started_at"`)] + `"started_at":""}`,
		"older schema":       strings.Replace(stored, `"schema_version":1`, `"schema_version":0`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if tampered == stored {
				t.Fatal("fixture tamper did not change the record")
			}
			f.put(t, pendingID, tampered)
			_, err := backend.Seal(context.Background(), f.seal(begin.TransitionID))
			wantError(t, err, "invalid native activation state")
			_, err = backend.Abort(context.Background(), f.abort(begin.TransitionID))
			wantError(t, err, "invalid native activation abort state")
		})
	}
}

func TestSealFailsWhenClockIsUnavailableAfterPendingValidated(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	begin, err := backend.Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	backend.now = nil
	_, err = backend.Seal(context.Background(), f.seal(begin.TransitionID))
	wantError(t, err, "native activation clock is unavailable")
	if _, ok := f.get(t, pendingID); !ok {
		t.Fatal("failed seal removed the pending record")
	}
	if _, ok := f.get(t, receiptID); ok {
		t.Fatal("failed seal wrote a receipt")
	}
}

func TestAbortWithoutPendingReportsMissingAndKeepsReceipt(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	begin, err := backend.Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Seal(context.Background(), f.seal(begin.TransitionID)); err != nil {
		t.Fatal(err)
	}
	_, err = backend.Abort(context.Background(), f.abort(begin.TransitionID))
	wantError(t, err, "native activation pending record is missing")
	if _, ok := f.get(t, receiptID); !ok {
		t.Fatal("abort of a sealed transition removed the receipt")
	}
}

func TestAbortReturnsPendingRecordFactsAndRefusesChangedBinary(t *testing.T) {
	f := newFixture(t)
	backend := testBackend(f.target)
	backend.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	begin, err := backend.Begin(context.Background(), f.begin())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.target, []byte("changed content!"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = backend.Abort(context.Background(), f.abort(begin.TransitionID))
	wantError(t, err, "invalid native activation abort state")
	if _, ok := f.get(t, pendingID); !ok {
		t.Fatal("refused abort deleted the pending record")
	}
	if err := os.WriteFile(f.target, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := backend.Abort(context.Background(), f.abort(begin.TransitionID))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Aborted || result.UpdatedAt != "2026-01-02T03:04:05Z" || result.BinarySHA256 != begin.BinarySHA256 ||
		result.StateRoot != f.root || result.TargetBinary != f.target {
		t.Fatalf("abort result = %+v", result)
	}
}

func TestBinaryIdentityRejectsUnsafeFiles(t *testing.T) {
	root := t.TempDir()
	const notExecutable = "activation binary must be an executable single-link regular file"

	plain := filepath.Join(root, "plain")
	if err := os.WriteFile(plain, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(plain, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := binaryIdentityFromPath(plain)
	wantError(t, err, notExecutable)

	directory := filepath.Join(root, "dir")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = binaryIdentityFromPath(directory)
	wantError(t, err, notExecutable)

	exe := filepath.Join(root, "exe")
	if err := os.WriteFile(exe, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(exe, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(exe, filepath.Join(root, "exe-hardlink")); err != nil {
		t.Fatal(err)
	}
	_, err = binaryIdentityFromPath(exe)
	wantError(t, err, notExecutable)

	if _, err := binaryIdentityFromPath(filepath.Join(root, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing path error = %v", err)
	}
}

func TestBinaryIdentityReportsContentHashModeAndPhysicalIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bin")
	if err := os.WriteFile(path, []byte("abc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	identity, err := binaryIdentityFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	const abcSHA256 = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	physical, _ := physicalIdentity(info)
	if identity.SHA256 != abcSHA256 || identity.Size != 3 || identity.Mode != uint32(info.Mode()) ||
		identity.Inode != physical.inode || identity.Device != physical.device || identity.Executable != path {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestHashStableBinaryRejectsIdentityMismatchAndMissingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bin")
	if err := os.WriteFile(path, []byte("abc"), 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	physical, _ := physicalIdentity(info)
	if _, err := hashStableBinary(path, info, physical); err != nil {
		t.Fatalf("stable file rejected: %v", err)
	}

	other := physical
	other.inode++
	_, err = hashStableBinary(path, info, other)
	wantError(t, err, "activation binary has no stable physical identity")

	if err := os.WriteFile(path, []byte("abcd"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = hashStableBinary(path, info, physical)
	wantError(t, err, "activation binary has no stable physical identity")

	if _, err := hashStableBinary(filepath.Join(root, "absent"), info, physical); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestDecodeExactRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	type sample struct {
		A int `json:"a"`
	}
	got, err := decodeExact[sample]([]byte(" {\"a\": 7}\n"))
	if err != nil || got.A != 7 {
		t.Fatalf("valid decode = %+v err=%v", got, err)
	}
	if _, err := decodeExact[sample]([]byte(`{"a":1,"b":2}`)); err == nil || !strings.Contains(err.Error(), `unknown field "b"`) {
		t.Fatalf("unknown field error = %v", err)
	}
	_, err = decodeExact[sample]([]byte(`{"a":1}{"a":2}`))
	wantError(t, err, "trailing native activation data")
	_, err = decodeExact[sample]([]byte(`{"a":1} garbage`))
	wantError(t, err, "trailing native activation data")
}

func TestNewTransitionIDProducesFreshValidIdentifiers(t *testing.T) {
	first, err := newTransitionID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newTransitionID()
	if err != nil {
		t.Fatal(err)
	}
	if !validTransitionID(first) || !validTransitionID(second) || first == second {
		t.Fatalf("ids = %q, %q", first, second)
	}
}
