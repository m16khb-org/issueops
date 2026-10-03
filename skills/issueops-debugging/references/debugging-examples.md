# Debugging Examples

Optional illustrations; [SKILL.md](../SKILL.md) owns the complete runtime contract.
Use only commands available in the target environment and preserve its authorization.


**A good diagnostic method works across any language, OS, or stack.** Apply the same evidence and isolation requirements in each environment.

### Language-Agnostic Debugging Commands

Every language has equivalents of these fundamental operations. You must know them for the stack you're working on:

| Operation | Go | Python | Node.js | Rust | Java |
|-----------|-----|--------|---------|------|------|
| **Run a test** | `go test -run TestX -count=1` | `pytest -k test_x` | `npx jest -t 'test x'` | `cargo test test_x` | `./gradlew test --tests XTest` |
| **Run with debug output** | `go test -v` | `pytest -v -s` | `NODE_DEBUG=module node` | `RUST_LOG=debug cargo run` | `-Dorg.slf4j.simpleLogger.defaultLog=debug` |
| **Stack trace** | Built into panic | `traceback.print_exc()` | `console.trace()` | `RUST_BACKTRACE=1` | `e.printStackTrace()` |
| **Profile CPU** | `go test -cpuprofile` | `cProfile` | `node --prof` | `perf record` | `jstack` + `jmap` |
| **Profile memory** | `go test -memprofile` | `memory_profiler` | `node --inspect` → Chrome | `heaptrack` | `jmap -histo` |
| **Inspect variable** | `fmt.Printf("%#v", x)` / `delve` | `print(repr(x))` / `pdb` | `console.dir(x, {depth: null})` | `dbg!(&x)` / `lldb` | `System.out.println(x)` |
| **Bisect tests** | `go test -run` + binary search | `pytest --stepwise` | `jest --testPathPattern` | `cargo test --test` | `./gradlew test --tests` |
| **Race detector** | `go test -race` | `pytest -x --timeout` (not native) | `--detectOpenHandles` | `Miri` / `ThreadSanitizer` | `jcstress` |

### Applying the Debugging Method Across Stacks

The 7-step method is language-agnostic. Translate each step to the target stack:

```
REPRODUCE:  Run the exact failing command in the target language's test runner.
TRANSLATE:  Pass the failure output to lint_diagnose (works across languages — it reads stderr).
ISOLATE:    Use the language's bisect/debug tools from the table above.
HYPOTHESIZE: State the hypothesis in plain English, independent of implementation language.
VERIFY:     Run the disproving test using the language's test runner.
FIX:        Apply the minimal fix using the language's idioms.
LEARN:      Record the diagnosis via self-augment lesson — also language-agnostic.
```

### Cross-Language Pattern Recognition

Some bug patterns transcend language boundaries. Recognize them regardless of syntax:

| Pattern | Go symptom | Python symptom | Node.js symptom | Root cause |
|---------|-----------|---------------|-----------------|-----------|
| N+1 | N DB calls in loop, visible in `-benchmem` allocs | Same, visible via `django-debug-toolbar` | Same, visible via `Sequelize.queryLog` | Missing eager load or `WHERE IN` |
| Race condition | `go test -race` WARNING: DATA RACE | `threading` + shared state, non-deterministic | `Promise` chain order unexpected | Missing lock or wrong lock ordering |
| Memory leak | `runtime.ReadMemStats` shows growing heap | `memory_profiler` shows unbounded growth | `process.memoryUsage()` grows monotonically | Unclosed resource, growing slice/map, forgotten goroutine |
| Infinite loop | CPU 100%, `pprof` shows single func dominating | Same, KeyboardInterrupt shows line | Same, process hangs | Loop invariant broken, input never matches exit condition |
| Off-by-one | Slice bounds panic at `len(x)` | `IndexError: list index out of range` | `undefined` at array boundary | `<=` where `<` needed, or vice versa |
| Nil/null dereference | Panic: `nil pointer dereference` | `AttributeError: 'NoneType' object has no attribute...` | `TypeError: Cannot read property... of null` | Missing nil check, wrong init order |
| Closed channel/socket | `panic: send on closed channel` | `OSError: [Errno 9] Bad file descriptor` | `ERR_STREAM_DESTROYED` | Resource closed before all writers finished |

---

## IssueOps Debugging Patterns Reference

| Failure pattern | Likely cause | First strategy |
|----------------|-------------|----------------|
| "It used to work" (regression) | Recent commit changed behavior | Strategy A: Bisect |
| "Everything is broken" (broad failure) | Config/init/infrastructure change | Strategy B: Divide & Conquer |
| "Sometimes it fails" (flaky) | Race condition, timeout, stale state | Strategy C: Trace Diff |
| "Expected X, got Y" (assertion) | Logic error in the assertion's code or test | Strategy B, then Step 2 |
| Golden/snapshot mismatch | Non-hermetic fixture or changed serialized output | Strategy D: Snapshot/Golden Diff |
| "Connection refused" / timeout | Service not running, port mismatch | Check process list, port bindings |
| "Import cycle" / build failure | Circular dependency introduced | Check git diff for new imports |
| "Panic / nil pointer" | Missing nil check, wrong init order | Read stack trace → locate line |
| Memory leak / performance regression | Unbounded resource accumulation | Profile: `go test -bench` with allocations |


## Lesson Recording Example

Record a Reflexion-style lesson so future debugging sessions can reference this pattern:

```bash
issueops self-augment lesson \
  --candidate self-verify-progress-heartbeat \
  --lesson "Auth middleware init before config load → 401 for all requests. Fix: ensure middleware init runs after config loader in the boot sequence." \
  --next-action "Audit all middleware init calls for config dependency ordering" \
  --severity warning \
  --json
```
