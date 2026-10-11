package lifecycle

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

const schema = 3

type fakeEffects struct {
	normalizeErr error
	profile      lifecyclecontract.ProjectLifecycleProfile
	readErrs     []error // consumed per ReadProfile call; empty => profile returned
	readCalls    int
	mkdirErr     error
	createErr    error
	writeErr     error
	now          time.Time

	calls   []string
	created *lifecyclecontract.ProjectLifecycleProfile
	written *lifecyclecontract.ProjectLifecycleProfile
}

func (f *fakeEffects) NormalizeRoot(root string) (string, error) {
	if f.normalizeErr != nil {
		return "", f.normalizeErr
	}
	return root, nil
}
func (f *fakeEffects) StateRoot() string { return "/state" }
func (f *fakeEffects) Fingerprint(root string) lifecyclecontract.ProjectFingerprint {
	return lifecyclecontract.ProjectFingerprint{RepoRoot: root, GitDir: root + "/.git"}
}
func (f *fakeEffects) Paths(root string, fp lifecyclecontract.ProjectFingerprint) lifecyclecontract.ProjectLifecycleStatePlan {
	return lifecyclecontract.ProjectLifecycleStatePlan{
		OK: true, SchemaVersion: schema, RepoRoot: root, RepoID: "id", StateRoot: "/state",
		ProjectStateDir: "/state/p", ProjectJSONPath: "/state/p/project.json", Fingerprint: fp,
	}
}
func (f *fakeEffects) ReadProfile(string) (lifecyclecontract.ProjectLifecycleProfile, error) {
	i := f.readCalls
	f.readCalls++
	if i < len(f.readErrs) && f.readErrs[i] != nil {
		return lifecyclecontract.ProjectLifecycleProfile{}, f.readErrs[i]
	}
	return f.profile, nil
}
func (f *fakeEffects) Mkdir(string) error { f.calls = append(f.calls, "mkdir"); return f.mkdirErr }
func (f *fakeEffects) Create(_ string, p lifecyclecontract.ProjectLifecycleProfile) error {
	f.calls = append(f.calls, "create")
	f.created = &p
	return f.createErr
}
func (f *fakeEffects) Write(_ string, p lifecyclecontract.ProjectLifecycleProfile) error {
	f.calls = append(f.calls, "write")
	f.written = &p
	return f.writeErr
}
func (f *fakeEffects) Now() time.Time { return f.now }

func validProfile(f *fakeEffects) lifecyclecontract.ProjectLifecycleProfile {
	return lifecyclecontract.ProjectLifecycleProfile{
		SchemaVersion: schema, RepoID: "id",
		Fingerprint: f.Fingerprint("/repo"), CreatedAt: "old-created", UpdatedAt: "old-updated",
	}
}

func TestResolveNormalizeErrorReturnsNotOKPlanWithStateRoot(t *testing.T) {
	boom := errors.New("bad root")
	plan, err := Service{Effects: &fakeEffects{normalizeErr: boom}, SchemaVersion: schema}.Resolve("x")
	if !errors.Is(err, boom) || plan.OK || plan.StateRoot != "/state" || plan.SchemaVersion != schema {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestResolveMissingProfileIsNotExistingAndNoWarning(t *testing.T) {
	f := &fakeEffects{readErrs: []error{fs.ErrNotExist}}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Resolve("/repo")
	if err != nil || plan.Exists || plan.Profile != nil || len(plan.Warnings) != 0 || !plan.OK {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestResolveUnreadableProfileWarnsWithoutError(t *testing.T) {
	f := &fakeEffects{readErrs: []error{errors.New("corrupt json")}}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Resolve("/repo")
	if err != nil || plan.Exists || !reflect.DeepEqual(plan.Warnings, []string{"project_json_read_error"}) {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestResolveValidatesNamespaceOfExistingProfile(t *testing.T) {
	f := &fakeEffects{}
	f.profile = validProfile(f)
	plan, _ := Service{Effects: f, SchemaVersion: schema}.Resolve("/repo")
	if !plan.Exists || !plan.NamespaceValid || len(plan.Warnings) != 0 {
		t.Fatalf("valid profile: %+v", plan)
	}
	f.profile.SchemaVersion = schema + 1
	plan, _ = Service{Effects: f, SchemaVersion: schema}.Resolve("/repo")
	if !plan.Exists || plan.NamespaceValid || !reflect.DeepEqual(plan.Warnings, []string{"namespace_mismatch"}) {
		t.Fatalf("schema mismatch: %+v", plan)
	}
}

func TestInitWithoutConfirmDoesNotTouchDisk(t *testing.T) {
	f := &fakeEffects{readErrs: []error{fs.ErrNotExist}}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", false)
	if err != nil || plan.Exists || len(f.calls) != 0 {
		t.Fatalf("plan=%+v err=%v calls=%v", plan, err, f.calls)
	}
}

func TestInitRefusesToOverwriteForeignNamespace(t *testing.T) {
	f := &fakeEffects{}
	f.profile = validProfile(f)
	f.profile.RepoID = "other"
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if err != nil || plan.NamespaceValid || len(f.calls) != 0 {
		t.Fatalf("plan=%+v err=%v calls=%v", plan, err, f.calls)
	}
}

func TestInitCreatesProfileWithUTCTimestampAndMetadata(t *testing.T) {
	f := &fakeEffects{
		readErrs: []error{fs.ErrNotExist},
		now:      time.Date(2026, 1, 2, 12, 0, 0, 5, time.FixedZone("KST", 9*3600)),
	}
	meta := projectdoccontract.ProjectProfile{}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true, meta)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"mkdir", "create"}) {
		t.Fatalf("calls = %v", f.calls)
	}
	want := "2026-01-02T03:00:00.000000005Z"
	if f.created.CreatedAt != want || f.created.UpdatedAt != want || f.created.SchemaVersion != schema || f.created.RepoID != "id" {
		t.Fatalf("created = %+v", f.created)
	}
	if f.created.Metadata == nil {
		t.Fatalf("metadata not persisted")
	}
	if !plan.Exists || !plan.NamespaceValid || plan.Profile == nil || plan.Profile.CreatedAt != want {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestInitMkdirFailureMarksPlanNotOK(t *testing.T) {
	boom := errors.New("mkdir failed")
	f := &fakeEffects{readErrs: []error{fs.ErrNotExist}, mkdirErr: boom}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if !errors.Is(err, boom) || plan.OK || len(f.calls) != 1 {
		t.Fatalf("plan=%+v err=%v calls=%v", plan, err, f.calls)
	}
}

func TestInitCreateFailureOtherThanExistIsReported(t *testing.T) {
	boom := errors.New("disk full")
	f := &fakeEffects{readErrs: []error{fs.ErrNotExist}, createErr: boom}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if !errors.Is(err, boom) || plan.OK || plan.Exists {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestInitLosingCreateRaceAdoptsWinnersProfile(t *testing.T) {
	f := &fakeEffects{createErr: fs.ErrExist, readErrs: []error{fs.ErrNotExist, nil}}
	f.profile = validProfile(f)
	f.profile.CreatedAt = "winner"
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if err != nil || !plan.Exists || !plan.NamespaceValid || plan.Profile.CreatedAt != "winner" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if f.written != nil {
		t.Fatalf("loser overwrote winner's profile")
	}
}

func TestInitLosingCreateRaceWithForeignNamespaceStaysInvalid(t *testing.T) {
	f := &fakeEffects{createErr: fs.ErrExist, readErrs: []error{fs.ErrNotExist, nil}}
	f.profile = validProfile(f)
	f.profile.RepoID = "other"
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if err != nil || plan.NamespaceValid || !reflect.DeepEqual(plan.Warnings, []string{"namespace_mismatch"}) {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestInitLosingCreateRaceReadFailureIsReported(t *testing.T) {
	boom := errors.New("read after race")
	f := &fakeEffects{createErr: fs.ErrExist, readErrs: []error{fs.ErrNotExist, boom}}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if !errors.Is(err, boom) || plan.OK {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestInitUpdatesExistingProfileKeepingCreatedAtAndMetadata(t *testing.T) {
	f := &fakeEffects{now: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)}
	f.profile = validProfile(f)
	f.profile.Metadata = &projectdoccontract.ProjectProfile{}
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"mkdir", "write"}) || f.created != nil {
		t.Fatalf("calls = %v", f.calls)
	}
	if f.written.CreatedAt != "old-created" || f.written.UpdatedAt != "2026-05-01T00:00:00Z" || f.written.Metadata == nil {
		t.Fatalf("written = %+v", f.written)
	}
	if !plan.NamespaceValid || plan.Profile.UpdatedAt != "2026-05-01T00:00:00Z" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestInitWriteFailureMarksPlanNotOK(t *testing.T) {
	boom := errors.New("write failed")
	f := &fakeEffects{writeErr: boom}
	f.profile = validProfile(f)
	plan, err := Service{Effects: f, SchemaVersion: schema}.Init("/repo", true)
	if !errors.Is(err, boom) || plan.OK {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
}

func TestInitPropagatesNormalizeError(t *testing.T) {
	boom := errors.New("bad root")
	f := &fakeEffects{normalizeErr: boom}
	_, err := Service{Effects: f, SchemaVersion: schema}.Init("x", true)
	if !errors.Is(err, boom) || len(f.calls) != 0 {
		t.Fatalf("err=%v calls=%v", err, f.calls)
	}
}
