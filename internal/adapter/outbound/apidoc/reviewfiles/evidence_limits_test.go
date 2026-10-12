package reviewfiles

import (
	"fmt"
	"strings"
	"testing"
)

// padToSize appends a comment so the returned text is exactly size bytes long.
func padToSize(content string, size int) string {
	n := size - len(content) - len("// \n")
	return content + "// " + strings.Repeat("x", n) + "\n"
}

func throwingService(class string) string {
	return fmt.Sprintf("export class %s {\n  run() {\n    throw new %sError();\n  }\n}\n", class, strings.TrimSuffix(class, "Service"))
}

func TestEvidenceIndexesServiceFilesUpToTheSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		want bool
	}{
		{"at the limit", evidenceMaxFileLen, true},
		{"one byte over", evidenceMaxFileLen + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeEvidenceRepo(t, map[string]string{
				"src/users.controller.ts": usersController,
				"src/users.service.ts":    padToSize("export class UsersService {\n  findOne() {\n    throw new NotFoundException();\n  }\n}\n", tc.size),
			})

			got := Evidence(dir, []string{"src/users.controller.ts"})

			if has := strings.Contains(got, "NotFoundException"); has != tc.want {
				t.Fatalf("service of %d bytes indexed = %v, want %v:\n%s", tc.size, has, tc.want, got)
			}
		})
	}
}

func TestEvidenceControllerExactlyAtTheSizeLimitIsParsed(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/users.controller.ts": padToSize(usersController, evidenceMaxFileLen),
		"svc/users.service.ts":    "export class UsersService {\n  findOne() {\n    throw new NotFoundException();\n  }\n}\n",
	})

	got := Evidence(dir, []string{"src/users.controller.ts"})

	want := "- src/users.controller.ts#findOne\n    findOne: throw new NotFoundException () (users.service.ts:3)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceClassIndexStopsAfterTheClassLimit(t *testing.T) {
	const service = "export class UsersService {\n  findOne() {\n    throw new NotFoundException();\n  }\n}\n"
	for _, tc := range []struct {
		name    string
		fillers int
		want    string
	}{
		{"limit reached before the service", evidenceMaxFiles, ""},
		{"one slot left for the service", evidenceMaxFiles - 1, "- c/users.controller.ts#findOne\n    findOne: throw new NotFoundException () (users.service.ts:3)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{
				"b/users.service.ts":    service,
				"c/users.controller.ts": usersController,
			}
			for i := range tc.fillers {
				files[fmt.Sprintf("a/f%04d.ts", i)] = fmt.Sprintf("export class Filler%04d {}\n", i)
			}

			got := Evidence(writeEvidenceRepo(t, files), []string{"c/users.controller.ts"})

			if got != tc.want {
				t.Fatalf("evidence =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

func TestEvidenceClassIndexKeepsEveryClassDeclaredInOneFile(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/users.controller.ts": usersController,
		"src/things.ts":           "export class FirstThing {}\nexport class UsersService {\n  findOne() {\n    throw new NotFoundException();\n  }\n}\n",
	})

	got := Evidence(dir, []string{"src/users.controller.ts"})

	want := "- src/users.controller.ts#findOne\n    findOne: throw new NotFoundException () (things.ts:4)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceInjectionParsingStopsWhereTheConstructorEnds(t *testing.T) {
	const tail = "  go() {\n%s  }\n}\n"
	cases := []struct {
		name  string
		ctor  string
		calls []string
		want  []string
	}{
		{
			name:  "single-line constructor ignores later properties",
			ctor:  "  constructor(private readonly a: AService) {}\n  private readonly stray: StrayService;\n",
			calls: []string{"a", "stray"},
			want:  []string{"A"},
		},
		{
			name:  "multi-line constructor keeps every parameter but not later properties",
			ctor:  "  constructor(\n    private readonly a: AService, private readonly b: BService,\n    private readonly d: DService,\n  ) {}\n  private readonly stray: StrayService;\n",
			calls: []string{"a", "b", "d", "stray"},
			want:  []string{"A", "B", "D"},
		},
		{
			name:  "unterminated constructor reads parameters up to the end of the file",
			ctor:  "  constructor(private readonly a: AService,\n    private readonly b: BService,\n",
			calls: []string{"a", "b"},
			want:  []string{"A", "B"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body strings.Builder
			for _, call := range tc.calls {
				fmt.Fprintf(&body, "    this.%s.run();\n", call)
			}
			files := map[string]string{
				"src/c.controller.ts": "export class C {\n" + tc.ctor + fmt.Sprintf(tail, body.String()),
			}
			for _, class := range []string{"A", "B", "D", "Stray"} {
				files["svc/"+strings.ToLower(class)+".service.ts"] = throwingService(class + "Service")
			}
			var want strings.Builder
			want.WriteString("- src/c.controller.ts#go\n")
			for _, class := range tc.want {
				fmt.Fprintf(&want, "    run: throw new %sError () (%s.service.ts:3)\n", class, strings.ToLower(class))
			}

			got := Evidence(writeEvidenceRepo(t, files), []string{"src/c.controller.ts"})

			if got != want.String() {
				t.Fatalf("evidence =\n%q\nwant\n%q", got, want.String())
			}
		})
	}
}

func TestEvidenceEmptyMethodBodyDoesNotSwallowTheNextMethod(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/c.controller.ts": `export class C {
  constructor(private readonly a: AService) {}
  noop() {}
  go() {
    this.a.run();
  }
}
`,
		"svc/a.service.ts": throwingService("AService"),
	})

	got := Evidence(dir, []string{"src/c.controller.ts"})

	want := "- src/c.controller.ts#go\n    run: throw new AError () (a.service.ts:3)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceReportsEveryMicroserviceSendOfAMethod(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gw/a.controller.ts": `export class A {
  constructor(private readonly client: ClientProxy) {}
  go() {
    this.client.send('first_one');
    this.client.send('second_one');
  }
}
`,
	})

	got := Evidence(dir, []string{"gw/a.controller.ts"})

	want := "- gw/a.controller.ts#go\n" +
		"    microservice hop 'first_one': no @MessagePattern/@EventPattern handler found in repo (remote service)\n" +
		"    microservice hop 'second_one': no @MessagePattern/@EventPattern handler found in repo (remote service)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceFollowsEverySameClassCallOfAServiceMethod(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/users.controller.ts": usersController,
		"src/users.service.ts": `export class UsersService {
  findOne() {
    this.first();
    this.second();
  }
  first() {
    throw new FirstError();
  }
  second() {
    throw new SecondError();
  }
}
`,
	})

	got := Evidence(dir, []string{"src/users.controller.ts"})

	want := "- src/users.controller.ts#findOne\n    first: throw new FirstError () (users.service.ts:7)\n    second: throw new SecondError () (users.service.ts:10)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceSkipsServiceCallsOnceMoreThanTwoHundredThrowsAreCollected(t *testing.T) {
	var service strings.Builder
	service.WriteString("export class S {\n  a() {\n")
	for range 200 {
		service.WriteString("    throw new E();\n")
	}
	service.WriteString("  }\n  b() {\n    throw new BError();\n  }\n  c() {\n    throw new CError();\n  }\n}\n")
	dir := writeEvidenceRepo(t, map[string]string{
		"src/c.controller.ts": `export class C {
  constructor(private readonly s: S) {}
  go() {
    this.s.a();
    this.s.b();
    this.s.c();
  }
}
`,
		"src/s.ts": service.String(),
	})

	got := Evidence(dir, []string{"src/c.controller.ts"})

	if n := strings.Count(got, "a: throw new E "); n != 200 {
		t.Errorf("a() throws reported = %d, want 200", n)
	}
	if !strings.Contains(got, "b: throw new BError") {
		t.Error("the call made at exactly 200 collected throws must still be followed")
	}
	if strings.Contains(got, "c: throw new CError") {
		t.Error("the call made after more than 200 collected throws must be skipped")
	}
}

func TestThrowDetailLengthAndCutBoundaries(t *testing.T) {
	words13 := strings.TrimSpace(strings.Repeat("word ", 13))
	cases := []struct {
		name, arg, want string
	}{
		{"exactly the maximum length is kept whole", "('" + strings.Repeat("a", 70), " ('" + strings.Repeat("a", 70)},
		{"separator at the minimum cut position is not used", strings.Repeat("x", 24) + " " + strings.Repeat("y", 75), " " + strings.Repeat("x", 24) + " " + strings.Repeat("y", 47) + "..."},
		{"closes only the unbalanced parentheses", "(a) ('" + strings.Repeat("word ", 20), " (a) ('" + words13 + "...)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := throwDetail(tc.arg); got != tc.want {
				t.Fatalf("throwDetail = %q, want %q", got, tc.want)
			}
		})
	}
}

const hopGateway = `export class G {
  constructor(private readonly client: ClientProxy) {}
  go() {
    this.client.send('job_run');
  }
}
`

const hopService = `export class JobsService {
  run() {
    throw new ConflictException('job exists');
  }
}
`

func workerController(decorator string) string {
	return "export class W {\n  constructor(private readonly jobsService: JobsService) {}\n" + decorator + "  handle() {\n    return this.jobsService.run();\n  }\n}\n"
}

func TestEvidenceHandlerMatchUsesOnlyTheDecoratorWindow(t *testing.T) {
	cases := []struct {
		name      string
		decorator string
		want      bool
	}{
		{"literal on a continuation line", "  @MessagePattern(\n    { a: 1 },\n    'job_run',\n  )\n", true},
		{"literal on the fourth line of the window", "  @MessagePattern({\n    a: 1,\n    b: 2,\n    cmd: 'job_run',\n  })\n", true},
		{"literal after the closing parenthesis is outside the window", "  @MessagePattern(\n    'other_pattern',\n  )\n  // job_run\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeEvidenceRepo(t, map[string]string{
				"gw/g.controller.ts":     hopGateway,
				"worker/w.controller.ts": workerController(tc.decorator),
				"worker/jobs.service.ts": hopService,
			})

			got := Evidence(dir, []string{"gw/g.controller.ts"})

			if !strings.Contains(got, "matched @MessagePattern/@EventPattern in worker/w.controller.ts") {
				t.Fatalf("file containing the literal must still be reported:\n%s", got)
			}
			if has := strings.Contains(got, "handler handle -> JobsService.run"); has != tc.want {
				t.Fatalf("handler evidence = %v, want %v:\n%s", has, tc.want, got)
			}
		})
	}
}

func TestEvidenceHopToUnterminatedDecoratorAtEndOfFile(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gw/g.controller.ts": hopGateway,
		"worker/w.ts":        "@MessagePattern({ cmd: 'job_run'",
	})

	got := Evidence(dir, []string{"gw/g.controller.ts"})

	want := "- gw/g.controller.ts#go\n    microservice hop 'job_run' matched @MessagePattern/@EventPattern in worker/w.ts\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceHopMatchesAnyLiteralOfTheSendPattern(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gw/g.controller.ts": `export class G {
  constructor(private readonly client: ClientProxy) {}
  go() {
    this.client.send({ cmd: 'first_one', alt: 'second_one' }, {});
  }
}
`,
		"worker/w.controller.ts": workerController("  @MessagePattern('second_one')\n"),
		"worker/jobs.service.ts": hopService,
	})

	got := Evidence(dir, []string{"gw/g.controller.ts"})

	if !strings.Contains(got, "handler handle -> JobsService.run") {
		t.Fatalf("handler must match through the second literal:\n%s", got)
	}
}

func TestEvidenceHopAcceptsTwoCharacterLiterals(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gw/g.controller.ts": `export class G {
  constructor(private readonly client: ClientProxy) {}
  go() {
    this.client.send('ab');
  }
}
`,
		"worker/w.ts": "@MessagePattern('ab')\nexport class W {}\n",
	})

	got := Evidence(dir, []string{"gw/g.controller.ts"})

	want := "- gw/g.controller.ts#go\n    microservice hop 'ab' matched @MessagePattern/@EventPattern in worker/w.ts\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceTruncationMarkerOnlyAboveTheByteBudget(t *testing.T) {
	const format = "microservice hop %s: no @MessagePattern/@EventPattern handler found in repo (remote service)"
	overhead := len(fmt.Sprintf(format, "''"))
	for _, tc := range []struct {
		name          string
		size          int
		wantTruncated bool
	}{
		{"exactly the budget", evidenceMaxBytes, false},
		{"one byte over the budget", evidenceMaxBytes + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			literal := strings.Repeat("x", tc.size-overhead)
			dir := writeEvidenceRepo(t, map[string]string{
				"gw/g.controller.ts": fmt.Sprintf("export class G {\n  constructor(private readonly client: ClientProxy) {}\n  go() {\n    this.client.send('%s');\n  }\n}\n", literal),
			})

			got := Evidence(dir, []string{"gw/g.controller.ts"})

			if !strings.Contains(got, fmt.Sprintf(format, "'"+literal+"'")) {
				t.Fatalf("hop line missing from evidence of %d bytes", len(got))
			}
			if has := strings.HasSuffix(got, "    (evidence truncated)\n"); has != tc.wantTruncated {
				t.Fatalf("truncation marker = %v, want %v", has, tc.wantTruncated)
			}
		})
	}
}
