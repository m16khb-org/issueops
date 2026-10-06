package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	core "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func childCLIRecoveryFixture(t *testing.T, provider string) (model.IssueOpsRecord, []string) {
	t.Helper()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	if provider == "gitlab" {
		record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/1234"
		record.BranchPrepare.Provider = "gitlab"
		record.BranchPrepare.IssueURL = record.IssueURL
		var err error
		record, err = (core.CycleRecordStore{StateRoot: issueOpsStateRootForTest()}).Save(context.Background(), record)
		if err != nil {
			t.Fatal(err)
		}
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for _, name := range []string{"gh", "glab"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!"+python+"\n"+childCLIFake), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CHILD_PARENT", record.IssueURL)
	return record, []string{"create-child", "--id", record.ID, "--title", "Child", "--body", readableChildBody, "--label", "bug", "--assignee", "octocat", "--confirm", "--json"}
}

func TestChildCLIImplicitResponseLossExplicitChildAndRestartReplay(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			record, args := childCLIRecoveryFixture(t, provider)
			var first app.ChildCreateResult
			err := testRemoteCommand().Run(args, Deps{PrintJSON: func(v any) error {
				b, _ := json.Marshal(v)
				_ = json.Unmarshal(b, &first)
				return errors.New("response lost after completed CAS")
			}})
			if err == nil || first.OperationID == "" {
				t.Fatalf("response loss=%v first=%+v", err, first)
			}
			replay := runChildCLI(t, args)
			if replay.OperationID != first.OperationID || replay.ChildURL != first.ChildURL {
				t.Fatalf("response loss replay=%+v first=%+v", replay, first)
			}
			second := runChildCLI(t, append(append([]string{}, args...), "--operation-id", strings.Repeat("f", 32)))
			if second.ChildURL == first.ChildURL || second.OperationID == first.OperationID {
				t.Fatal("explicit creation conflated")
			}
			replay = runChildCLI(t, args)
			if replay.ChildURL != first.ChildURL || replay.OperationID != first.OperationID {
				t.Fatal("explicit child replaced implicit identity")
			}
			assertChildCLICount(t, record.Repo, 2)
			persisted, err := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
			if err != nil || persisted.IssueURL != record.IssueURL || len(persisted.IssueLinks) != 2 {
				t.Fatalf("record=%+v err=%v", persisted, err)
			}
		})
	}
}
func TestChildCLIAmbiguousErrorJSONPreviewAndReconcile(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			record, args := childCLIRecoveryFixture(t, provider)
			t.Setenv("CHILD_FAULT", "after-create")
			var failure struct {
				app.ChildCreateResult
				Error string `json:"error"`
			}
			err := testRemoteCommand().Run(args, Deps{PrintJSON: func(v any) error { b, _ := json.Marshal(v); return json.Unmarshal(b, &failure) }})
			if err == nil || failure.OK || failure.ChildURL == "" || failure.OperationID == "" || !strings.Contains(failure.RecoveryCommand, "reconcile-child") {
				t.Fatalf("error JSON=%+v err=%v", failure, err)
			}
			if err := testRemoteCommand().Run(args, Deps{PrintJSON: func(any) error { return nil }}); err == nil {
				t.Fatal("ambiguous request retried")
			}
			before, _ := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
			bytesBefore, _ := json.Marshal(before)
			t.Setenv("CHILD_FAULT", "")
			reconcile := []string{"reconcile-child", "--id", record.ID, "--operation-id", failure.OperationID, "--json"}
			if err := testRemoteCommand().Run(reconcile, Deps{PrintJSON: func(any) error { return nil }}); err != nil {
				t.Fatal(err)
			}
			after, _ := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
			bytesAfter, _ := json.Marshal(after)
			if string(bytesBefore) != string(bytesAfter) {
				t.Fatal("preview changed durable record")
			}
			if err := testRemoteCommand().Run(append(reconcile, "--confirm"), Deps{PrintJSON: func(any) error { return nil }}); err != nil {
				t.Fatal(err)
			}
			replay := runChildCLI(t, args)
			if replay.ChildURL != failure.ChildURL || replay.OperationID != failure.OperationID {
				t.Fatal("reconcile lost child identity")
			}
			assertChildCLICount(t, record.Repo, 1)
		})
	}
}

func TestChildCLIStaleOperationRequiresCurrentHolderReconcile(t *testing.T) {
	record, args := childCLIRecoveryFixture(t, "github")
	worktree, ancestry := activateRemoteIssueOpsRecordForCurrentProcess(t, &record)
	actorFlags := []string{"--host", "codex", "--session-id", "session-1", "--cwd", worktree}
	args = append(args, actorFlags...)
	deps := Deps{ObserveProcessAncestry: func(int) ([]model.NativeProcessReceipt, error) { return ancestry, nil }, PrintJSON: func(any) error { return nil }}
	if err := testRemoteCommand().Run(args, deps); err != nil {
		t.Fatal(err)
	}
	record, err := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	operation := record.ChildCreateOperations[0]
	record.Execution.Lease.Generation++
	record, err = (core.CycleRecordStore{StateRoot: issueOpsStateRootForTest()}).Save(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	if err := testRemoteCommand().Run(args, deps); err == nil || !strings.Contains(err.Error(), "explicit reconcile") {
		t.Fatalf("stale auto-replay=%v", err)
	}
	reconcile := append([]string{"reconcile-child", "--id", record.ID, "--operation-id", operation.OperationID, "--confirm", "--json"}, actorFlags...)
	wrong := append([]string(nil), reconcile...)
	for i, value := range wrong {
		if value == "session-1" {
			wrong[i] = "foreign-session"
		}
	}
	if err := testRemoteCommand().Run(wrong, deps); err == nil {
		t.Fatal("foreign reconcile accepted")
	}
	if err := testRemoteCommand().Run(reconcile, deps); err != nil {
		t.Fatal(err)
	}
	if err := testRemoteCommand().Run(args, deps); err != nil {
		t.Fatal(err)
	}
	assertChildCLICount(t, record.Repo, 1)
	updated, err := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
	if err != nil || updated.ChildCreateOperations[0].Generation != record.Execution.Lease.Generation {
		t.Fatalf("rebind not persisted: %v", err)
	}
}
func runChildCLI(t *testing.T, args []string) app.ChildCreateResult {
	t.Helper()
	var result app.ChildCreateResult
	err := testRemoteCommand().Run(args, Deps{PrintJSON: func(v any) error { b, _ := json.Marshal(v); return json.Unmarshal(b, &result) }})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func assertChildCLICount(t *testing.T, repo string, count int) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "children.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []any
	if err := json.Unmarshal(b, &rows); err != nil || len(rows) != count {
		t.Fatalf("create count=%d want=%d err=%v", len(rows), count, err)
	}
}

const childCLIFake = `import json,sys,os,re,pathlib
a=sys.argv[1:]
p=pathlib.Path('children.json')
rows=json.loads(p.read_text()) if p.exists() else []
parent=os.environ['CHILD_PARENT']
hierarchy_parent=os.environ.get('CHILD_HIERARCHY_PARENT',parent)
def output(v):
 print(json.dumps(v));sys.exit(0)
def value(flag):
 return a[a.index(flag)+1] if flag in a else ''
def field(key):
 return next((x[len(key)+1:] for x in a if x.startswith(key+'=')), '')
def item(row):
 return dict(id='gid://gitlab/WorkItem/'+row['iid'],iid=row['iid'],webUrl=row['url'],title=row['title'],description=row['body'],workItemType=dict(name='Task'),widgets=[dict(type='HIERARCHY',hasParent=row['attached'],parent=dict(id='gid://gitlab/WorkItem/1234',webUrl=hierarchy_parent) if row['attached'] else None),dict(type='LABELS',labels=dict(nodes=[dict(title='bug')])),dict(type='ASSIGNEES',assignees=dict(nodes=[dict(username='octocat')]))])
def save():
 p.write_text(json.dumps(rows))
def create(title,body,preferred):
 iid=str(34+len(rows))
 row=dict(iid=iid,url=parent.rsplit('/',1)[0]+'/'+iid,title=title,body=body,attached=preferred)
 rows.append(row);save();return row
if a[:2]==['issue','create']:
 if '--help' in a: print('  --parent string');sys.exit(0)
 row=create(value('--title'),value('--body'),'--parent' in a)
 print(row['url'])
 sys.exit(1 if os.environ.get('CHILD_FAULT') else 0)
if a[:2]==['issue','list']:
 output([dict(url=r['url'],title=r['title'],body=r['body']) for r in rows if value('--search') in r['body']])
if a and a[0]=='api' and 'graphql' not in a:
 if 'POST' in a:
  rows[-1]['attached']=True;save();output(dict(ok=True))
 endpoint=a[1]
 if 'sub_issues' in endpoint: output([dict(id=987+i,number=int(r['iid']),html_url=r['url']) for i,r in enumerate(rows) if r['attached']])
 r=next((r for r in rows if endpoint.endswith('/'+r['iid'])),None)
 if r: output(dict(id=987+rows.index(r),number=int(r['iid']),html_url=r['url'],title=r['title'],body=r['body'],labels=[dict(name='bug')],assignees=[dict(login='octocat')]))
q=field('query')
if 'taskType' in q: output(dict(data=dict(namespace=dict(workItemTypes=dict(nodes=[dict(id='gid://gitlab/WorkItems::Type/2',name='Task')])))))
if 'labelLookup' in q: output(dict(data=dict(project=dict(labels=dict(nodes=[dict(id='gid://gitlab/ProjectLabel/7',title='bug')])))))
if 'userLookup' in q: output(dict(data=dict(user=dict(id='gid://gitlab/User/9',username='octocat'))))
if 'workItemCreate(' in q:
 title=json.loads(re.search(r'title:[ ]*("(?:\\.|[^"\\])*")',q).group(1))
 body=json.loads(re.search(r'description:[ ]*("(?:\\.|[^"\\])*")',q).group(1))
 row=create(title,body,False)
 print(json.dumps(dict(data=dict(workItemCreate=dict(workItem=item(row),errors=[])))))
 if os.environ.get('CHILD_FAULT')=='follow-up-start':
  pathlib.Path(sys.argv[0]).write_text('#!/missing-issueops-test-interpreter\n')
  sys.exit(0)
 sys.exit(1 if os.environ.get('CHILD_FAULT') else 0)
if 'childRecoverySearch' in q: output(dict(data=dict(project=dict(workItems=dict(nodes=[item(r) for r in rows if field('marker') in r['body']],pageInfo=dict(hasNextPage=False))))))
if 'childRecovery(' in q: output(dict(data=dict(project=dict(workItems=dict(nodes=[item(r) for r in rows if r['iid']==field('childIid')],pageInfo=dict(hasNextPage=False))))))
if 'workItemHierarchyAddChildrenItems(' in q:
 for r in rows:
  if field('childId').endswith('/'+r['iid']): r['attached']=True
 save();output(dict(data=dict(workItemHierarchyAddChildrenItems=dict(errors=[]))))
if 'parentIid' in q: output(dict(data=dict(project=dict(issue=dict(id='gid://gitlab/WorkItem/1234')))))
if 'query children' in q: output(dict(data=dict(workItem=dict(widgets=[dict(type='HIERARCHY',children=dict(nodes=[item(r) for r in rows if r['attached']]))]))))
if 'query childVerify' in q:
 r=next(r for r in rows if field('childId').endswith('/'+r['iid']));output(dict(data=dict(workItem=item(r))))
print('unexpected provider call',file=sys.stderr);sys.exit(2)
`

func TestChildCLIFollowUpStartFailurePersistsKnownURL(t *testing.T) {
	record, args := childCLIRecoveryFixture(t, "gitlab")
	executable, err := exec.LookPath("glab")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHILD_FAULT", "follow-up-start")
	var failure app.ChildCreateResult
	err = testRemoteCommand().Run(args, Deps{PrintJSON: func(v any) error { b, _ := json.Marshal(v); return json.Unmarshal(b, &failure) }})
	if err == nil || failure.ChildURL == "" {
		t.Fatalf("failure=%+v err=%v", failure, err)
	}
	// A new production reader must retain the receipt despite the next process failing to start.
	persisted, err := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	op := persisted.ChildCreateOperations[0]
	if op.CanonicalURL != failure.ChildURL || op.Status == model.IssueCreateIntentNotInvoked {
		t.Fatalf("lost durable receipt: %+v error=%v", op, err)
	}
	if err = os.WriteFile(executable, original, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHILD_FAULT", "")
	if err = testRemoteCommand().Run(args, Deps{PrintJSON: func(any) error { return nil }}); err == nil {
		t.Fatal("partial operation retried")
	}
	reconcile := []string{"reconcile-child", "--id", record.ID, "--operation-id", op.OperationID, "--confirm", "--json"}
	if err = testRemoteCommand().Run(reconcile, Deps{PrintJSON: func(any) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	replay := runChildCLI(t, args)
	if replay.ChildURL != failure.ChildURL {
		t.Fatal("changed recovered URL")
	}
	assertChildCLICount(t, record.Repo, 1)
}

func TestChildCLIGitLabParentAliasesAndDrift(t *testing.T) {
	for _, identity := range []string{"alias", "host", "project", "iid"} {
		t.Run(identity, func(t *testing.T) {
			record, args := childCLIRecoveryFixture(t, "gitlab")
			t.Setenv("CHILD_FAULT", "after-create")
			var failure app.ChildCreateResult
			if err := testRemoteCommand().Run(args, Deps{PrintJSON: func(v any) error { b, _ := json.Marshal(v); return json.Unmarshal(b, &failure) }}); err == nil {
				t.Fatal("expected create response loss")
			}
			rowsPath := filepath.Join(record.Repo, "children.json")
			data, err := os.ReadFile(rowsPath)
			if err != nil {
				t.Fatal(err)
			}
			var rows []map[string]any
			if err = json.Unmarshal(data, &rows); err != nil {
				t.Fatal(err)
			}
			rows[0]["attached"] = true
			data, _ = json.Marshal(rows)
			if err = os.WriteFile(rowsPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			parent := strings.Replace(record.IssueURL, "/work_items/", "/issues/", 1)
			switch identity {
			case "host":
				parent = strings.Replace(parent, "gitlab.example.com", "other.example.com", 1)
			case "project":
				parent = strings.Replace(parent, "acme/repo", "acme/other", 1)
			case "iid":
				parent = strings.Replace(parent, "/1234", "/9999", 1)
			}
			t.Setenv("CHILD_HIERARCHY_PARENT", parent)
			t.Setenv("CHILD_FAULT", "")
			before, err := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			reconcile := []string{"reconcile-child", "--id", record.ID, "--operation-id", failure.OperationID, "--json"}
			err = testRemoteCommand().Run(reconcile, Deps{PrintJSON: func(any) error { return nil }})
			after, readErr := core.ReadIssueOps(issueOpsStateRootForTest(), record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			a, _ := json.Marshal(before)
			b, _ := json.Marshal(after)
			if string(a) != string(b) {
				t.Fatal("preview wrote record")
			}
			if identity != "alias" {
				if err == nil {
					t.Fatal("foreign parent accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = testRemoteCommand().Run(append(reconcile, "--confirm"), Deps{PrintJSON: func(any) error { return nil }}); err != nil {
				t.Fatal(err)
			}
			assertChildCLICount(t, record.Repo, 1)
			unchanged, err := os.ReadFile(rowsPath)
			if err != nil || string(unchanged) != string(data) {
				t.Fatalf("already attached alias triggered remote write: %v", err)
			}
		})
	}
}
