package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	auditcontract "issueops/internal/contract/audit"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestCommandWriterAppendsOwnerOnlyJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "audit", "log.jsonl")
	writer := CommandWriter{Filename: path}
	for _, id := range []string{"first", "second"} {
		if err := writer.Append(path, auditcontract.CommandAuditRecord{OK: true, Kind: "command_policy_audit", AuditLogID: id}); err != nil {
			t.Fatal(err)
		}
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil || dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("audit directory must be owner-only: %v %v", dirInfo, err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("audit log must be owner-only: %v %v", fileInfo, err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var ids []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record auditcontract.CommandAuditRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("each record must be one JSON line: %v", err)
		}
		ids = append(ids, record.AuditLogID)
	}
	if strings.Join(ids, ",") != "first,second" {
		t.Fatalf("records must append in order, got %v", ids)
	}
}

func TestCommandWriterAppendFailsWhenLogLocationIsNotWritable(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := auditcontract.CommandAuditRecord{OK: true}
	if err := (CommandWriter{}).Append(filepath.Join(blocker, "audit.jsonl"), record); err == nil {
		t.Fatal("a regular file where the audit directory belongs must fail")
	}
	directoryLog := filepath.Join(root, "is-a-dir")
	if err := os.Mkdir(directoryLog, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := (CommandWriter{}).Append(directoryLog, record); err == nil {
		t.Fatal("a directory in place of the log file must fail")
	}
}

func TestCommandWriterPathReportsResolution(t *testing.T) {
	want := os.ErrInvalid
	path, err := (CommandWriter{Filename: "/x/log.jsonl", ResolveError: want}).Path()
	if path != "/x/log.jsonl" || err != want {
		t.Fatalf("path=%q err=%v", path, err)
	}
	logPath := filepath.Join(t.TempDir(), "log.jsonl")
	t.Setenv("ISSUEOPS_AUDIT_LOG", logPath)
	path, err = NewCommandWriter().Path()
	if err != nil || path != logPath {
		t.Fatalf("NewCommandWriter must resolve the environment path: %q %v", path, err)
	}
}

func TestCommandAuditLogPathResolvesRelativeAndHomeFallbacks(t *testing.T) {
	t.Setenv("ISSUEOPS_AUDIT_LOG", "relative/audit.jsonl")
	got, err := commandAuditLogPath()
	if err != nil || !filepath.IsAbs(got) || !strings.HasSuffix(got, filepath.Join("relative", "audit.jsonl")) {
		t.Fatalf("relative override must become absolute: %q %v", got, err)
	}

	home := t.TempDir()
	t.Setenv("ISSUEOPS_AUDIT_LOG", "")
	t.Setenv("ISSUEOPS_STATE_DIR", "")
	t.Setenv("HOME", home)
	got, err = commandAuditLogPath()
	if want := filepath.Join(home, ".local", "state", "issueops", "audit", "command-policy.jsonl"); err != nil || got != want {
		t.Fatalf("home fallback=%q want %q err=%v", got, want, err)
	}

	t.Setenv("HOME", "")
	if got, err := commandAuditLogPath(); err == nil || got != "" || !strings.Contains(err.Error(), "resolve home for audit log") {
		t.Fatalf("missing home must fail: %q %v", got, err)
	}
}

func TestBoundedHandoffDeliveryFieldTrimsAndTruncatesAtLimit(t *testing.T) {
	if got := boundedHandoffDeliveryField("  value \n"); got != "value" {
		t.Fatalf("got %q", got)
	}
	exact := strings.Repeat("a", handoffDeliveryFieldLimit)
	if got := boundedHandoffDeliveryField(exact); got != exact {
		t.Fatal("a field exactly at the limit must be kept whole")
	}
	got := boundedHandoffDeliveryField(exact + "b")
	if got != exact+"..." {
		t.Fatalf("over-limit field must be cut at the limit and marked: len=%d", len(got))
	}
}

func TestHandoffDeliveryHandleRejectsNilAndUnverifiableUse(t *testing.T) {
	var nilHandle *handoffDeliveryAuditHandle
	if err := nilHandle.Close(); err != nil {
		t.Fatalf("closing a nil handle must be a no-op: %v", err)
	}
	if err := nilHandle.VerifyPath(); err == nil {
		t.Fatal("nil handle verified")
	}
	if err := (&handoffDeliveryAuditHandle{}).VerifyPath(); err == nil {
		t.Fatal("handle without verifier verified")
	}
	if err := (&handoffDeliveryAuditHandle{}).Close(); err != nil {
		t.Fatalf("empty handle close: %v", err)
	}
}

func TestRedactedHandoffDeliveryObservationScrubsEveryExecutablePathWithoutMutatingInput(t *testing.T) {
	secret := "token=owner-secret " + strings.Repeat("z", handoffDeliveryFieldLimit+10)
	observation := auditDeliveryObservationFixture()
	observation.OwnerActor = &issueopscontract.NativeActor{SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 1, Executable: secret}}
	observation.OwnerClaim.Actor.SessionProcess = &issueopscontract.NativeProcessReceipt{PID: 2, Executable: secret}

	got := redactedHandoffDeliveryObservation(observation)
	for name, value := range map[string]string{
		"owner actor": got.OwnerActor.SessionProcess.Executable,
		"owner claim": got.OwnerClaim.Actor.SessionProcess.Executable,
	} {
		if strings.Contains(value, "owner-secret") || len(value) > handoffDeliveryFieldLimit+3 {
			t.Fatalf("%s executable not redacted and bounded (len %d)", name, len(value))
		}
	}
	if observation.OwnerActor.SessionProcess.Executable != secret || observation.OwnerClaim.Actor.SessionProcess.Executable != secret {
		t.Fatal("redaction mutated the caller's nested process receipts")
	}
	if got.OwnerActor == observation.OwnerActor {
		t.Fatal("redacted observation must not alias the caller's actor")
	}
}

func TestHandoffDeliveryAuditErrorAffectsOnlyReadersOfTheSameLineage(t *testing.T) {
	record := func(lifecycle, lineage string) auditcontract.HandoffDeliveryAuditRecord {
		return auditcontract.HandoffDeliveryAuditRecord{Observation: issueopscontract.IssueOpsHandoffDeliveryObservation{LifecycleID: lifecycle, LineageID: lineage}}
	}
	for _, tt := range []struct {
		name                       string
		record                     auditcontract.HandoffDeliveryAuditRecord
		readerLifecycle, readerLin string
		want                       bool
	}{
		{"unscoped reader sees every corruption", record("a", "b"), "", "", true},
		{"same lineage", record("a", "b"), "a", "b", true},
		{"other lifecycle", record("a", "b"), "x", "b", false},
		{"other lineage", record("a", "b"), "a", "x", false},
		{"unidentifiable record poisons scoped readers (lifecycle)", record("", "b"), "a", "b", true},
		{"unidentifiable record poisons scoped readers (lineage)", record("a", ""), "a", "b", true},
	} {
		if got := handoffDeliveryAuditErrorAffects(tt.record, tt.readerLifecycle, tt.readerLin); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}

func TestHandoffDeliveryStateRootPathCanonicalizesOnlyTrustedAliases(t *testing.T) {
	if _, err := handoffDeliveryStateRootPath("relative/state"); err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("relative root: %v", err)
	}
	if _, err := handoffDeliveryStateRootPath("   "); err == nil {
		t.Fatal("blank root accepted")
	}
	got, err := handoffDeliveryStateRootPath("  /opt/issueops/../issueops/state/ ")
	if err != nil || got != "/opt/issueops/state" {
		t.Fatalf("untrusted prefix must only be cleaned: %q %v", got, err)
	}
	if runtime.GOOS != "darwin" {
		if got, err := handoffDeliveryStateRootPath("/tmp/state"); err != nil || got != "/tmp/state" {
			t.Fatalf("no alias is trusted off darwin: %q %v", got, err)
		}
		return
	}
	for input, want := range map[string]string{
		"/tmp/issueops/state":  "/private/tmp/issueops/state",
		"/var/issueops/state":  "/private/var/issueops/state",
		"/tmpfoo/state":        "/tmpfoo/state",
		"/variable/state":      "/variable/state",
		"/private/tmp/x/state": "/private/tmp/x/state",
	} {
		if got, err := handoffDeliveryStateRootPath(input); err != nil || got != want {
			t.Fatalf("%s -> %q (%v), want %q", input, got, err, want)
		}
	}
}

func writeAuditLog(t *testing.T, stateRoot string, mutate func(lines []string) []string) string {
	t.Helper()
	store := auditStoreForTest(stateRoot)
	if _, err := store.Append(auditDeliveryObservationFixture()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := mutate(strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHandoffDeliveryReadRejectsEachIntegrityFailure(t *testing.T) {
	for name, tt := range map[string]struct {
		mutate func(*auditcontract.HandoffDeliveryAuditRecord)
		want   string
	}{
		"not ok":          {func(r *auditcontract.HandoffDeliveryAuditRecord) { r.OK = false }, "invalid kind"},
		"wrong kind":      {func(r *auditcontract.HandoffDeliveryAuditRecord) { r.Kind = "other" }, "invalid kind"},
		"wrong schema":    {func(r *auditcontract.HandoffDeliveryAuditRecord) { r.SchemaVersion = 2 }, "invalid kind"},
		"missing digest":  {func(r *auditcontract.HandoffDeliveryAuditRecord) { r.RecordDigest = "" }, "digest mismatch"},
		"tampered digest": {func(r *auditcontract.HandoffDeliveryAuditRecord) { r.RecordDigest = strings.Repeat("0", 64) }, "digest mismatch"},
		"tampered body": {func(r *auditcontract.HandoffDeliveryAuditRecord) {
			r.Observation.Launcher.Name = "other"
		}, "digest mismatch"},
		"receipt location": {func(r *auditcontract.HandoffDeliveryAuditRecord) {
			r.Observation.Receipt.Location = "audit/elsewhere.json"
			r.RecordDigest = handoffDeliveryRecordDigest(*r)
		}, "receipt mismatch"},
		"receipt digest": {func(r *auditcontract.HandoffDeliveryAuditRecord) {
			r.Observation.Receipt.Digest = strings.Repeat("1", 64)
			r.RecordDigest = handoffDeliveryRecordDigest(*r)
		}, "receipt mismatch"},
		"domain invalid": {func(r *auditcontract.HandoffDeliveryAuditRecord) {
			r.Observation.SourceGeneration = 0
			r.Observation.Receipt.Digest = handoffDeliveryReceiptDigest(r.AuditLogID, r.Observation)
			r.RecordDigest = handoffDeliveryRecordDigest(*r)
		}, "source generation"},
	} {
		t.Run(name, func(t *testing.T) {
			stateRoot := t.TempDir()
			writeAuditLog(t, stateRoot, func(lines []string) []string {
				var record auditcontract.HandoffDeliveryAuditRecord
				if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
					t.Fatal(err)
				}
				tt.mutate(&record)
				encoded, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				lines[0] = string(encoded)
				return lines
			})
			observations, err := auditStoreForTest(stateRoot).Read()
			if err == nil || !strings.Contains(err.Error(), tt.want) || len(observations) != 0 {
				t.Fatalf("observations=%d err=%v want %q", len(observations), err, tt.want)
			}
		})
	}
}

func TestHandoffDeliveryReadForFiltersByLineageAndPoisonsOnUnidentifiableCorruption(t *testing.T) {
	stateRoot := t.TempDir()
	store := auditStoreForTest(stateRoot)
	sameLifecycleOtherLineage := auditDeliveryObservationFixture()
	sameLifecycleOtherLineage.LineageID = "lineage-2"
	sameLifecycleOtherLineage.AttemptID = "attempt-2"
	for _, observation := range []issueopscontract.IssueOpsHandoffDeliveryObservation{auditDeliveryObservationFixture(), sameLifecycleOtherLineage} {
		if _, err := store.Append(observation); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.ReadFor("io-delivery", "lineage-2")
	if err != nil || len(got) != 1 || got[0].AttemptID != "attempt-2" {
		t.Fatalf("lineage filter: %+v %v", got, err)
	}
	if got, err := store.ReadFor("io-delivery", "lineage-missing"); err != nil || len(got) != 0 {
		t.Fatalf("unknown lineage: %+v %v", got, err)
	}

	path := filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"ok":false}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadFor("io-delivery", "lineage-1")
	if err == nil || !strings.Contains(err.Error(), "invalid kind") || len(got) != 1 {
		t.Fatalf("an unidentifiable corrupt record must fail scoped reads while keeping the verified prefix: %+v %v", got, err)
	}
}

func TestHandoffDeliveryAppendRejectsInvalidObservationWithoutCreatingLog(t *testing.T) {
	stateRoot := t.TempDir()
	observation := auditDeliveryObservationFixture()
	observation.SourceGeneration = 0
	record, err := auditStoreForTest(stateRoot).Append(observation)
	if err == nil || record.AuditLogID != "" {
		t.Fatalf("invalid observation recorded: %+v %v", record, err)
	}
	if _, statErr := os.Stat(filepath.Join(stateRoot, "audit")); !os.IsNotExist(statErr) {
		t.Fatalf("rejected observation must not touch disk: %v", statErr)
	}
}

func TestHandoffDeliveryAppendRejectsRecordBeyondReaderLimit(t *testing.T) {
	stateRoot := t.TempDir()
	store := auditStoreForTest(stateRoot)
	record := auditcontract.HandoffDeliveryAuditRecord{OK: true, Kind: "handoff_delivery_observation", SchemaVersion: 1}
	record.Observation.Launcher.Name = strings.Repeat("x", 256*1024)
	if err := store.append(record); err == nil || !strings.Contains(err.Error(), "exceeds reader limit") {
		t.Fatalf("oversized record: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "audit")); !os.IsNotExist(err) {
		t.Fatalf("oversized record must be refused before opening the log: %v", err)
	}
}

func TestHandoffDeliveryReadTreatsMissingStateAsEmptyButNotUnsafeState(t *testing.T) {
	root := t.TempDir()
	if got, err := auditStoreForTest(filepath.Join(root, "missing")).Read(); err != nil || len(got) != 0 {
		t.Fatalf("missing state root: %+v %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := auditStoreForTest(filepath.Join(root, "empty")).Read(); err != nil || len(got) != 0 {
		t.Fatalf("missing audit directory: %+v %v", got, err)
	}
	withAuditDir := filepath.Join(root, "withdir")
	if err := os.MkdirAll(filepath.Join(withAuditDir, "audit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := auditStoreForTest(withAuditDir).Read(); err != nil || len(got) != 0 {
		t.Fatalf("missing log: %+v %v", got, err)
	}
	if _, err := auditStoreForTest("relative").Read(); err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("relative root: %v", err)
	}
	if _, err := auditStoreForTest("/").Read(); err == nil || !strings.Contains(err.Error(), "filesystem root") {
		t.Fatalf("filesystem root: %v", err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := auditStoreForTest(file).Read(); err == nil {
		t.Fatal("a regular file as state root was accepted")
	}
	if _, err := auditStoreForTest(filepath.Join(root, "missing")).Append(auditDeliveryObservationFixture()); err == nil {
		t.Fatal("append must not create the state root itself")
	}
}

func TestHandoffDeliveryAuditRejectsUnsafeAuditDirectoryAndLeafKinds(t *testing.T) {
	t.Run("audit directory permissions", func(t *testing.T) {
		stateRoot := t.TempDir()
		auditDir := filepath.Join(stateRoot, "audit")
		if err := os.Mkdir(auditDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(auditDir, 0o755); err != nil {
			t.Fatal(err)
		}
		store := auditStoreForTest(stateRoot)
		if _, err := store.Read(); err == nil || !strings.Contains(err.Error(), "audit directory") {
			t.Fatalf("read: %v", err)
		}
		if _, err := store.Append(auditDeliveryObservationFixture()); err == nil || !strings.Contains(err.Error(), "audit directory") {
			t.Fatalf("append: %v", err)
		}
	})
	t.Run("audit path is a file", func(t *testing.T) {
		stateRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(stateRoot, "audit"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		store := auditStoreForTest(stateRoot)
		if _, err := store.Read(); err == nil {
			t.Fatal("read accepted a file as the audit directory")
		}
		if _, err := store.Append(auditDeliveryObservationFixture()); err == nil {
			t.Fatal("append accepted a file as the audit directory")
		}
	})
	t.Run("leaf is a directory", func(t *testing.T) {
		stateRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl"), 0o700); err != nil {
			t.Fatal(err)
		}
		store := auditStoreForTest(stateRoot)
		if _, err := store.Read(); err == nil || !strings.Contains(err.Error(), "unsafe type or permissions") {
			t.Fatalf("read: %v", err)
		}
		if _, err := store.Append(auditDeliveryObservationFixture()); err == nil || !strings.Contains(err.Error(), "unsafe type or permissions") {
			t.Fatalf("append: %v", err)
		}
	})
	t.Run("leaf replaced between inspection and open", func(t *testing.T) {
		stateRoot := t.TempDir()
		leaf := filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")
		if err := os.MkdirAll(filepath.Dir(leaf), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(leaf, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		store := auditStoreForTest(stateRoot)
		store.openHooks.beforeLeafOpen = func() {
			if err := os.Rename(leaf, leaf+".old"); err != nil {
				panic(err)
			}
			if err := os.WriteFile(leaf, nil, 0o600); err != nil {
				panic(err)
			}
		}
		if _, err := store.Read(); err == nil || !strings.Contains(err.Error(), "changed while opening") {
			t.Fatalf("leaf swap must be detected: %v", err)
		}
	})
}

func TestBoundedHandoffDeliveryFieldNeverSplitsMultibyteRunes(t *testing.T) {
	// 한 is 3 bytes and 1024 is not a multiple of 3, so the limit falls mid-rune.
	got := boundedHandoffDeliveryField(strings.Repeat("한", handoffDeliveryFieldLimit))
	if !utf8.ValidString(got) {
		t.Fatalf("truncation produced invalid UTF-8: %q", got[len(got)-8:])
	}
	if len(got) > handoffDeliveryFieldLimit+3 || !strings.HasSuffix(got, "...") {
		t.Fatalf("len=%d suffix=%q", len(got), got[len(got)-3:])
	}
}
