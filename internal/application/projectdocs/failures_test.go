package projectdocs

import (
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

var errEffect = errors.New("effect failed")

type failingRevision struct {
	revisionEffects
	readErr, writeErr error
}

func (fake *failingRevision) Read(path string) (string, bool, error) {
	if fake.readErr != nil {
		return "", false, fake.readErr
	}
	return fake.revisionEffects.Read(path)
}

func (fake *failingRevision) Write(path, content string) error {
	if fake.writeErr != nil {
		return fake.writeErr
	}
	return fake.revisionEffects.Write(path, content)
}

type failingAppend struct {
	appendEffects
	ensureErr, writeErr, lineErr error
}

func (fake *failingAppend) EnsureDir(path string) error {
	if fake.ensureErr != nil {
		return fake.ensureErr
	}
	return fake.appendEffects.EnsureDir(path)
}

func (fake *failingAppend) Write(path, content string) error {
	if fake.writeErr != nil {
		return fake.writeErr
	}
	return fake.appendEffects.Write(path, content)
}

func (fake *failingAppend) AppendLine(path, line string) error {
	if fake.lineErr != nil {
		return fake.lineErr
	}
	return fake.appendEffects.AppendLine(path, line)
}

type failingRoute struct {
	routeEffects
	dirErr  error
	fileErr error
}

func (fake *failingRoute) ReadDir(dir string) ([]string, error) {
	if fake.dirErr != nil {
		return nil, fake.dirErr
	}
	return fake.routeEffects.ReadDir(dir)
}

func (fake *failingRoute) ReadFile(path string) (string, error) {
	content, _ := fake.routeEffects.ReadFile(path)
	return content, fake.fileErr
}

type readFails struct{ readEffects }

func (readFails) Read(string) (string, bool, error) { return "", false, errEffect }

func TestReadPropagatesReadFailure(t *testing.T) {
	if _, err := Read("/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", readFails{}); !errors.Is(err, errEffect) {
		t.Fatalf("err=%v", err)
	}
}

func TestReviseRejectsInvalidInputBeforeTouchingTheFile(t *testing.T) {
	cases := map[string]struct {
		request projectdocscontract.ProjectDocsReviseRequest
		want    string
	}{
		"blank content": {projectdocscontract.ProjectDocsReviseRequest{Content: " \n", Summary: "why"}, "content is required"},
		"blank summary": {projectdocscontract.ProjectDocsReviseRequest{Content: "# Doc", Summary: "  "}, "summary is required"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &failingRevision{readErr: errEffect}
			_, err := Revise(tc.request, "/repo", ".issueops/TESTING.md", "/repo/.issueops/TESTING.md", fake)
			if err == nil || !strings.Contains(err.Error(), tc.want) || errors.Is(err, errEffect) {
				t.Fatalf("err=%v, want validation error %q before any read", err, tc.want)
			}
		})
	}
}

func TestRevisePropagatesReadAndWriteFailures(t *testing.T) {
	request := projectdocscontract.ProjectDocsReviseRequest{Content: "# Doc", Summary: "why", Confirm: true}
	if _, err := Revise(request, "/repo", ".issueops/TESTING.md", "/p", &failingRevision{readErr: errEffect}); !errors.Is(err, errEffect) {
		t.Fatalf("read err=%v", err)
	}
	fake := &failingRevision{writeErr: errEffect}
	if _, err := Revise(request, "/repo", ".issueops/TESTING.md", "/p", fake); !errors.Is(err, errEffect) {
		t.Fatalf("write err=%v", err)
	}
	// A dry run must never reach the failing Write.
	request.Confirm = false
	if result, err := Revise(request, "/repo", ".issueops/TESTING.md", "/p", fake); err != nil || !result.DryRun {
		t.Fatalf("dry run result=%+v err=%v", result, err)
	}
}

func TestAppendRejectsInvalidRequestsWithoutTouchingDisk(t *testing.T) {
	cases := map[string]struct {
		request projectdocscontract.ProjectDocsAppendRequest
		want    string
	}{
		"unknown kind":  {projectdocscontract.ProjectDocsAppendRequest{Kind: "note", Title: "T", Summary: "S"}, "unsupported record kind"},
		"blank title":   {projectdocscontract.ProjectDocsAppendRequest{Kind: "adr", Title: " ", Summary: "S"}, "title is required"},
		"blank summary": {projectdocscontract.ProjectDocsAppendRequest{Kind: "adr", Title: "T", Summary: ""}, "summary is required"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &appendEffects{}
			_, err := Append("/repo", tc.request, fake)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(fake.events) != 0 {
				t.Fatalf("err=%v events=%v", err, fake.events)
			}
		})
	}
}

func TestAppendStopsOnDirectoryAndWriteFailures(t *testing.T) {
	request := projectdocscontract.ProjectDocsAppendRequest{Kind: "adr", Title: "Choice", Summary: "why"}
	fake := &failingAppend{ensureErr: errEffect}
	if _, err := Append("/repo", request, fake); !errors.Is(err, errEffect) || len(fake.events) != 0 {
		t.Fatalf("ensure err=%v events=%v", err, fake.events)
	}
	fake = &failingAppend{writeErr: errEffect, appendEffects: appendEffects{existing: map[string]bool{"/repo/.issueops/adr/overview.md": true}}}
	if _, err := Append("/repo", request, fake); !errors.Is(err, errEffect) || len(fake.lines) != 0 {
		t.Fatalf("write err=%v lines=%v: overview must not be linked when the record was not written", err, fake.lines)
	}
}

func TestAppendReportsOverviewLinkFailureAsWarningAfterWritingRecord(t *testing.T) {
	fake := &failingAppend{lineErr: errEffect, appendEffects: appendEffects{existing: map[string]bool{"/repo/.issueops/cautions/overview.md": true}}}
	result, err := Append("/repo", projectdocscontract.ProjectDocsAppendRequest{Kind: "caution", Title: "Disk", Summary: "why"}, fake)
	if err != nil {
		t.Fatalf("record is written, link failure must not be an error: %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "could not link .issueops/cautions/2000-01-01-disk.md") || !strings.Contains(result.Warnings[0], errEffect.Error()) {
		t.Fatalf("warnings=%v", result.Warnings)
	}
}

func TestRouteWarnsWhenRecordsCannotBeListedOrRead(t *testing.T) {
	dir := &failingRoute{dirErr: fs.ErrPermission, routeEffects: routeEffects{existing: map[string]bool{"/repo/.issueops": true}}}
	result := Route("/repo", "docker", dir)
	// "docker" routes the default docs, so CONVENTIONS and TESTING module dirs are
	// listed too. Every directory that cannot be listed is reported exactly once,
	// including cautions/, which is both a record and a module directory.
	listed := map[string]int{}
	for _, warning := range result.Warnings {
		if rest, ok := strings.CutPrefix(warning, "could not list .issueops/"); ok && strings.Contains(warning, fs.ErrPermission.Error()) {
			listed[strings.Fields(rest)[0]]++
		}
	}
	want := map[string]int{"cautions": 1, "adr": 1, "conventions": 1, "testing": 1}
	if !maps.Equal(listed, want) {
		t.Fatalf("want one list warning per unreadable directory %v, got %v (warnings=%v)", want, listed, result.Warnings)
	}

	file := &failingRoute{fileErr: errEffect, routeEffects: routeEffects{
		existing: map[string]bool{"/repo/.issueops": true},
		files:    map[string]string{"/repo/.issueops/adr/2026-09-01-docker.md": "# Docker image build\n"},
	}}
	// Sanity: the same record is routed when it can be read, so the absence
	// below is caused by the read failure.
	healthy := &failingRoute{routeEffects: file.routeEffects}
	if routed := Route("/repo", "docker image build", healthy); !slices.ContainsFunc(routed.Docs, func(d projectdocscontract.ProjectDocRouteEntry) bool {
		return strings.Contains(d.RelPath, "2026-09-01-docker")
	}) {
		t.Fatalf("fixture record must match the task when readable: %+v", routed.Docs)
	}
	result = Route("/repo", "docker image build", file)
	if len(result.Warnings) != 1 || result.Warnings[0] != "could not read record .issueops/adr/2026-09-01-docker.md: "+errEffect.Error() {
		t.Fatalf("warnings=%v", result.Warnings)
	}
	for _, doc := range result.Docs {
		if strings.Contains(doc.RelPath, "2026-09-01-docker") {
			t.Fatalf("unreadable record must not be routed: %+v", doc)
		}
	}
}

func serviceUnderTest() Service {
	routing := &routeEffects{existing: map[string]bool{}}
	return Service{
		NormalizeRoot: func(root string) (string, error) {
			if root == "" {
				return "", errEffect
			}
			return "/norm/" + root, nil
		},
		Revision:  &revisionEffects{current: "# Old\n", exists: true},
		Routing:   routing,
		Appending: &appendEffects{existing: map[string]bool{}},
	}
}

func TestServiceNormalizesRootBeforeEveryOperation(t *testing.T) {
	service := serviceUnderTest()
	if _, err := service.Read("", "TESTING.md"); !errors.Is(err, errEffect) {
		t.Fatalf("Read err=%v", err)
	}
	if _, err := service.Revise(projectdocscontract.ProjectDocsReviseRequest{Content: "x", Summary: "y"}); !errors.Is(err, errEffect) {
		t.Fatalf("Revise err=%v", err)
	}
	if _, err := service.Route("", "task"); !errors.Is(err, errEffect) {
		t.Fatalf("Route err=%v", err)
	}
	if _, err := service.Append(projectdocscontract.ProjectDocsAppendRequest{Kind: "adr", Title: "T", Summary: "S"}); !errors.Is(err, errEffect) {
		t.Fatalf("Append err=%v", err)
	}
}

func TestServiceReadAndReviseRejectUnknownDocsAndResolveAllowedOnes(t *testing.T) {
	service := serviceUnderTest()
	if _, err := service.Read("repo", "secrets.md"); err == nil || !strings.Contains(err.Error(), "unsupported project doc") {
		t.Fatalf("Read err=%v", err)
	}
	if _, err := service.Revise(projectdocscontract.ProjectDocsReviseRequest{RepoRoot: "repo", RelPath: "../x.md", Content: "x", Summary: "y"}); err == nil || !strings.Contains(err.Error(), "unsupported project doc") {
		t.Fatalf("Revise err=%v", err)
	}
	read, err := service.Read("repo", "TESTING.md")
	if err != nil || read.RepoRoot != "/norm/repo" || read.RelPath != ".issueops/TESTING.md" || read.Path != "/norm/repo/.issueops/TESTING.md" || read.Content != "# Old\n" {
		t.Fatalf("read=%+v err=%v", read, err)
	}
	revised, err := service.Revise(projectdocscontract.ProjectDocsReviseRequest{RepoRoot: "repo", RelPath: ".issueops/TESTING.md", Content: "# New", Summary: "y", ExpectedSHA256: projectdocdomain.SHA256Hex("# Old\n")})
	if err != nil || revised.Path != "/norm/repo/.issueops/TESTING.md" || revised.Action != "update" || !revised.DryRun {
		t.Fatalf("revised=%+v err=%v", revised, err)
	}
}

func TestServiceRouteAndAppendUseNormalizedRoot(t *testing.T) {
	service := serviceUnderTest()
	routed, err := service.Route("repo", "refactor")
	if err != nil || routed.RepoRoot != "/norm/repo" {
		t.Fatalf("routed=%+v err=%v", routed, err)
	}
	appended, err := service.Append(projectdocscontract.ProjectDocsAppendRequest{RepoRoot: "repo", Kind: "adr", Title: "Choice", Summary: "why"})
	if err != nil || appended.RepoRoot != "/norm/repo" || appended.Path != "/norm/repo/.issueops/adr/2000-01-01-choice.md" {
		t.Fatalf("appended=%+v err=%v", appended, err)
	}
}
