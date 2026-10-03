# Optional measurement recipes

[Code Quality Metrics](../SKILL.md) owns scope, thresholds, snapshots, and gates.
These examples are approximations, not a semantic skill runner. Adapt file lists,
language boundaries, and installed tools; record the exact command used.

## Go function boundaries and redundancy candidates

```bash
for file in *.go; do
  [ -f "$file" ] || continue
  rg -n '^func ' "$file" | while IFS=: read -r line signature; do
    body=$(sed -n "${line},/^}/p" "$file")
    printf '%s:%s:%s:%s\n' "$file" "$line" \
      "$(printf '%s\n' "$body" | wc -l)" "$signature"
  done
done
```

This locates top-level Go function bodies ending at a top-level closing brace.
Similar lengths in adjacent functions are candidates only: inspect token overlap
or use AST tooling to measure duplication. Python `def`, JS/TS functions/arrows,
and Rust `fn` need different start and end patterns.
For entropy, inspect each extracted body and count branch points using the
entrypoint's definition, including boolean operators; line counts are not branch counts.

## Per-file overhead estimate (Bash)

```bash
FILE_LIST=$(mktemp)
trap 'rm -f "$FILE_LIST"' EXIT
git diff --name-only -z HEAD > "$FILE_LIST" || exit
git ls-files --others --exclude-standard -z >> "$FILE_LIST" || exit
while IFS= read -r -d '' f; do
  [ -f "$f" ] || continue
  TOTAL=$(wc -l < "$f")
  [ "$TOTAL" -gt 0 ] || { echo "SKIP EMPTY: $f"; continue; }
  BOILER=$(command grep -cE '^[[:space:]]*(import|package|@|//|/\*|type.*struct|func.*return|func \(.*\) .*return)' "$f")
  grep_status=$?
  case "$grep_status" in
    0) : ;;
    1) BOILER=0 ;;
    *) exit "$grep_status" ;;
  esac
  LOGIC=$((TOTAL - BOILER))
  OVERHEAD=$(echo "scale=2; $BOILER / $TOTAL" | bc)
  echo "$f: boilerplate=$BOILER logic=$LOGIC overhead=$OVERHEAD"
done < "$FILE_LIST"
```

Restrict the inventory to source files before scoring. This pattern can misclassify
type contracts and meaningful comments; report that limitation rather than calling
the result an exact count. Quote paths and use NUL-delimited inventories.

## Pre-PR tools

- Source size: count source lines per file, excluding tests; flag >250 LOC.
- Nesting: prefer an AST-aware language tool; flag >4. A count of `if`/`for`
  occurrences in the first 80 lines is only a warning heuristic, not nesting depth.
- Coverage: `go test -cover ./...`, `pytest --cov --cov-report=term`,
  `npx jest --coverage`, or `cargo tarpaulin`; flag packages <60%.
- Dead code: run an installed/project-local tool such as `staticcheck ./...`.
  Preserve its exit status and all findings before filtering U1000.
  If unavailable, report that or ask before global installation.

Use the same measured scope and commands before/after. This catalog does not
replace baseline, target-card, gate-result, or feedback-recording obligations.
