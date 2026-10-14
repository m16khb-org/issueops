# Worktree state at the commit-push stage

Branch `12-gates-failed-summary`, upstream `origin/12-gates-failed-summary` (pushed once at stage 4).

## git status --short

```text
 M cmd/issueops/gatescli/gates.go
 M internal/contract/gates/types.go
 M internal/domain/gates/check_result.go
 M internal/domain/gates/check_result_test.go
 M README.md
?? .env.local
?? .issueops/issues/12/verified-execution-report.md
```

## git diff --stat

```text
 README.md                                  |  2 +-
 cmd/issueops/gatescli/gates.go             |  3 +++
 internal/contract/gates/types.go           |  2 ++
 internal/domain/gates/check_result.go      |  6 ++++++
 internal/domain/gates/check_result_test.go | 24 ++++++++++++++++++++++++
```

- The four Go files add `failed_gates` to `gates check --json` and a `FAILED:` line to the human output, with a test.
- `README.md` fixes an unrelated typo ("recieve" → "receive") the developer noticed while reading.
- `.env.local` holds `GITHUB_TOKEN=...` for local provider testing.
- The verified-execution report belongs to this cycle (stage 7 wrote it).

## Verification already run on this tree (stage 7)

```text
$ gofmt -l $(git ls-files '*.go')
(no output)
$ go test ./internal/domain/gates/ ./cmd/issueops/gatescli/ -count=1
ok  	issueops/internal/domain/gates	0.29s
ok  	issueops/cmd/issueops/gatescli	0.41s
```

`go vet ./...`, `go test -race ./...`, and the response-contract golden were not run.

## git config --show-origin --get-regexp '^user\.'

```text
file:/Users/dev/.gitconfig	user.name Dev Kim
file:/Users/dev/.gitconfig	user.email dev.kim@corp-internal.example
```
