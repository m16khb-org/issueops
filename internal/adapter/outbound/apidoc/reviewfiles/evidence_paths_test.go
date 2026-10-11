package reviewfiles

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const usersController = `import { Controller, Get } from '@nestjs/common';
@Controller('users')
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Get(':id')
  findOne(@Param('id') id: string) {
    return this.usersService.findOne(id);
  }
}
`

func TestEvidenceFollowsOneSameClassHopWithLineNumbers(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/users.controller.ts": usersController,
		"src/users.service.ts": `@Injectable()
export class UsersService {
  findOne(id: string) {
    this.validate(id);
    return id;
  }
  validate(id: string) {
    if (!id) {
      throw new BadRequestException('bad id');
    }
    this.deep();
  }
  deep() {
    throw new GoneException();
  }
}
`,
	})

	got := Evidence(dir, []string{"src/users.controller.ts"})

	want := "- src/users.controller.ts#findOne\n    validate: throw new BadRequestException ('bad id') (users.service.ts:9)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q (a second hop to deep() must not be followed)", got, want)
	}
}

func TestEvidenceIgnoresFilesThatAreNotReadableTypeScriptControllers(t *testing.T) {
	huge := usersController + "// " + strings.Repeat("x", evidenceMaxFileLen) + "\n"
	service := `export class UsersService {
  findOne(id: string) {
    throw new NotFoundException('nope');
  }
}
`
	// repo is a subdirectory so that "../" and absolute-looking paths point at real controllers
	// that only the readRepoFile guard keeps out of the evidence.
	base := writeEvidenceRepo(t, map[string]string{
		"users.controller.ts":              usersController,
		"repo/src/users.controller.ts":     usersController,
		"repo/src/users.controller.js":     usersController,
		"repo/src/users.helper.ts":         usersController,
		"repo/src/huge.controller.ts":      huge,
		"repo/abs/users.controller.ts":     usersController,
		"repo/src/users.service.ts":        service,
		"repo/src/skip.controller.ts":      strings.ReplaceAll(usersController, "UsersService", "SkippedService"),
		"repo/src/dist/skipped.service.ts": strings.ReplaceAll(service, "UsersService", "SkippedService"),
	})
	dir := filepath.Join(base, "repo")
	if got := Evidence(dir, []string{"src/users.controller.ts"}); !strings.Contains(got, "NotFoundException") {
		t.Fatalf("baseline controller produced no evidence: %q", got)
	}
	for _, file := range []string{"src/users.controller.js", "src/users.helper.ts", "src/huge.controller.ts", "src/absent.controller.ts", "../users.controller.ts", "/abs/users.controller.ts", "src/skip.controller.ts"} {
		if got := Evidence(dir, []string{file}); got != "" {
			t.Errorf("Evidence(%q) = %q, want empty", file, got)
		}
	}
}

func TestEvidenceSkipsUnresolvableInjectionsClientsAndBodylessMethods(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/mix.controller.ts": `export class MixController {
  constructor(
    private readonly ghostService: GhostService,
    private readonly bus: EventBusClient,
    private readonly logger: Logger,
  ) {}

  declared(): void;

  a() {
    this.ghostService.run();
    this.bus.send('x_y');
    this.unknown.run();
  }
}
`,
		"src/bus.ts": `export class EventBusClient {
  send() {
    throw new Error('transport down');
  }
}
`,
	})
	// GhostService is declared nowhere; EventBusClient is a client type; `unknown` is not injected.
	// The send('x_y') hop still reports that no handler exists.
	got := Evidence(dir, []string{"src/mix.controller.ts"})
	want := "- src/mix.controller.ts#a\n    microservice hop 'x_y': no @MessagePattern/@EventPattern handler found in repo (remote service)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
}

func TestEvidenceHopLinksHandlersAndExceptionFilters(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gateway/jobs.controller.ts": `export class JobsController {
  constructor(private readonly client: ClientProxy) {}

  @Post()
  run() {
    this.client.emit({ cmd: 'job_run' }, {});
  }
}
`,
		"worker/jobs.controller.ts": `export class WorkerJobs {
  constructor(private readonly jobsService: JobsService, private readonly client: ClientProxy) {}

  @EventPattern({
    cmd: 'job_run',
  })
  handle(data: any) {
    this.client.send('other');
    return this.jobsService.run(data);
  }

  @MessagePattern('unrelated_pattern')
  other() {
    return this.jobsService.never();
  }

  @MessagePattern({ cmd: 'job_run_dup' })
  // pattern literal shared but no method follows within the window
}
`,
		"worker/jobs.service.ts": `export class JobsService {
  run(data: any) {
    throw new ConflictException('job exists');
  }
  never() {
    throw new GoneException();
  }
}
`,
		"worker/filters/rpc.filter.ts": `@Catch(RpcException)
export class RpcFilter {
  catch(e: any, host: any) {
    host.status(409).json({});
    const s = HttpStatus.BAD_REQUEST;
    host.status(409);
  }
}
`,
		"worker/filters/plain.ts": `@Catch(HttpException)
export class Plain {
  catch() {}
}
`,
	})

	got := Evidence(dir, []string{"gateway/jobs.controller.ts"})

	want := strings.Join([]string{
		"- gateway/jobs.controller.ts#run",
		"    microservice hop { cmd: 'job_run' } matched @MessagePattern/@EventPattern in worker/jobs.controller.ts",
		"      handler handle -> JobsService.run (worker/jobs.service.ts)",
		"    run: throw new ConflictException ('job exists') (jobs.service.ts:3)",
		"    exception filter @Catch(RpcException) in worker/filters/rpc.filter.ts maps to 409, HttpStatus.BAD_REQUEST, 409",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("evidence =\n%s\nwant\n%s", got, want)
	}
}

func TestEvidenceCapsMatchedHandlerFilesAtFour(t *testing.T) {
	files := map[string]string{
		"gw/a.controller.ts": `export class A {
  constructor(private readonly client: ClientProxy) {}
  go() {
    this.client.send('shared_pattern');
  }
}
`,
	}
	for i := range 6 {
		files[fmt.Sprintf("remote/h%d.ts", i)] = "@MessagePattern('shared_pattern')\nexport class H {}\n"
	}

	got := Evidence(writeEvidenceRepo(t, files), []string{"gw/a.controller.ts"})

	if n := strings.Count(got, "microservice hop"); n != 4 {
		t.Fatalf("matched handler files = %d, want 4:\n%s", n, got)
	}
	if strings.Contains(got, "h4.ts") || strings.Contains(got, "h5.ts") {
		t.Fatalf("files are visited in sorted order, h4/h5 must be cut off:\n%s", got)
	}
}

func TestEvidenceIgnoresHopPatternsWithoutUsableLiterals(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"gw/a.controller.ts": `export class A {
  constructor(private readonly client: ClientProxy) {}
  go() {
    this.client.send('a');
  }
}
`,
	})
	if got := Evidence(dir, []string{"gw/a.controller.ts"}); got != "" {
		t.Fatalf("one-character literals are too weak to match, got %q", got)
	}
}

func TestEvidenceTruncatesAtTheByteBudget(t *testing.T) {
	var service strings.Builder
	service.WriteString("export class BigService {\n  run() {\n")
	for i := range 150 {
		fmt.Fprintf(&service, "    throw new CustomException%d('%s');\n", i, strings.Repeat("m", 40))
	}
	service.WriteString("  }\n}\n")
	dir := writeEvidenceRepo(t, map[string]string{
		"src/big.controller.ts": `export class BigController {
  constructor(private readonly bigService: BigService) {}
  go() {
    return this.bigService.run();
  }
}
`,
		"src/big.service.ts": service.String(),
	})

	got := Evidence(dir, []string{"src/big.controller.ts"})

	if !strings.HasSuffix(got, "    (evidence truncated)\n") {
		t.Fatalf("evidence must end with the truncation marker, tail: %q", got[max(0, len(got)-80):])
	}
	if strings.Contains(got, "CustomException149") {
		t.Fatal("lines beyond the budget must be dropped")
	}
}

func TestEvidenceClassIndexKeepsFirstDeclarationInPathOrderAndSkipsHiddenDirs(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/users.controller.ts":  usersController,
		"src/a/users.service.ts":   "export class UsersService {\n  findOne() {\n    throw new FirstException();\n  }\n}\n",
		"src/b/users.service.ts":   "export class UsersService {\n  findOne() {\n    throw new SecondException();\n  }\n}\n",
		".hidden/users.service.ts": "export class UsersService {\n  findOne() {\n    throw new HiddenException();\n  }\n}\n",
	})

	got := Evidence(dir, []string{"src/users.controller.ts"})

	if !strings.Contains(got, "FirstException") || strings.Contains(got, "SecondException") || strings.Contains(got, "HiddenException") {
		t.Fatalf("expected only the first declaration in walk order:\n%s", got)
	}
}

func TestThrowDetailBoundsAndBalancesLongArguments(t *testing.T) {
	words := strings.TrimSpace(strings.Repeat("word ", 14))
	cases := []struct {
		name, arg, want string
	}{
		{"empty", "  ", ""},
		{"short keeps text and drops trailing comma", "('bad id'),  ", " ('bad id')"},
		{"long call is cut at a word and closed", "('" + strings.Repeat("word ", 20), " ('" + words + "...)"},
		{"long single-quoted text is closed", "'" + strings.Repeat("word ", 20), " '" + words + "...'"},
		{"long double-quoted text is closed", `"` + strings.Repeat("word ", 20), ` "` + words + `..."`},
		{"no separator past the minimum keeps the byte cut", strings.Repeat("x", 100), " " + strings.Repeat("x", 72) + "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := throwDetail(tc.arg); got != tc.want {
				t.Fatalf("throwDetail = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractMethodBodyHandlesBraceOnLaterLineAndUnterminatedBodies(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		ok    bool
		body  string
	}{
		{"brace on next line", []string{"m(a,", "  b)", "{", "  x;", "}", "after"}, true, "{\n  x;\n}"},
		{"declaration without body", []string{"m(a): void;", "{"}, false, ""},
		{"no brace at all", []string{"m(a)", "b"}, false, ""},
		{"unbalanced body runs to end of file", []string{"m() {", "  if (x) {", "  y"}, true, "m() {\n  if (x) {\n  y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, body, ok := extractMethodBody(tc.lines, 0)
			if ok != tc.ok || body != tc.body {
				t.Fatalf("extractMethodBody = %q, %v; want %q, %v", body, ok, tc.body, tc.ok)
			}
		})
	}
}

func TestEvidenceControlFlowStatementsAreNotMethods(t *testing.T) {
	dir := writeEvidenceRepo(t, map[string]string{
		"src/c.controller.ts": `export class C {
  constructor(private readonly s: S) {}
  go(id: string) {
    if (id) {
      return this.s.run();
    }
    for (const x of [1]) {
      this.s.run();
    }
  }
}
`,
		"src/s.ts": "export class S {\n  run() {\n    throw new E();\n  }\n}\n",
	})

	got := Evidence(dir, []string{"src/c.controller.ts"})

	want := "- src/c.controller.ts#go\n    run: throw new E () (s.ts:3)\n    run: throw new E () (s.ts:3)\n"
	if got != want {
		t.Fatalf("evidence =\n%q\nwant\n%q", got, want)
	}
	for _, bogus := range []string{"#if", "#for"} {
		if strings.Contains(got, bogus) {
			t.Fatalf("control flow reported as method %s:\n%s", bogus, got)
		}
	}
}
