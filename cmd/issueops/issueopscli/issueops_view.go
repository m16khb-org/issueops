package issueopscli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

// runIssueOpsView는 cycle 한 건의 조망을 읽는 표면이다. 읽기 전용이며 record나
// state를 바꾸지 않는다. `--view timeline`과 `--view graph`를 받는다. `--cwd`는
// graph 조망이 `next` 판정을 내릴 위치이며 비우면 현재 디렉터리를 쓴다.
// timeline 조망은 `--cwd`를 쓰지 않는다.
func (cli command) runIssueOpsView(args []string) error {
	fs := flag.NewFlagSet("issueops view", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	view := fs.String("view", "", "view to render (timeline|graph)")
	cwd := fs.String("cwd", "", "working directory for views that resolve a worktree")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	if cli.Runtime.IssueOpsView == nil || cli.Runtime.IssueOpsStateRoot == nil {
		return errors.New("issueops runtime is not configured")
	}
	directory := strings.TrimSpace(*cwd)
	if directory == "" && *view == issueopsviewcontract.ViewGraph {
		current, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve working directory: %w", err)
		}
		directory = current
	}
	result, err := cli.Runtime.IssueOpsView(cli.Runtime.IssueOpsStateRoot(), directory, *id, *view)
	if err != nil {
		// 다른 issueops 명령과 같은 오류 형태를 낸다: --json이면 {"ok":false,"error":...}.
		if *jsonOut {
			return reportIssueOpsError(err)
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Graph != nil {
		printIssueOpsViewGraphText(result)
		return nil
	}
	printIssueOpsViewText(result)
	return nil
}

// printIssueOpsViewGraphText prints the graph header, one line per edge, and the
// reasons a reader must not take a "not_reported" edge for a fresh one.
func printIssueOpsViewGraphText(result issueopsviewcontract.Result) {
	graph := result.Graph
	stale, unknown := 0, 0
	for _, edge := range graph.Edges {
		switch edge.State {
		case issueopsviewcontract.EdgeStateStale:
			stale++
		case issueopsviewcontract.EdgeStateUnknown:
			unknown++
		}
	}
	fmt.Printf("graph: stage=%s stale=%d unknown=%d\n", viewTextField(graph.JudgedBy.Stage), stale, unknown)
	if graph.UnknownReason != "" {
		fmt.Printf("  unknown_reason: %s\n", graph.UnknownReason)
	}
	if len(graph.JudgedBy.UnmappedStaleKeys) > 0 {
		fmt.Printf("  unmapped stale: %s\n", strings.Join(graph.JudgedBy.UnmappedStaleKeys, ", "))
	}
	for _, edge := range graph.Edges {
		fmt.Printf("  %s -> %s  [%s]  %s\n", edge.From, edge.To, viewTextField(edge.State), viewTextField(edge.Gate))
	}
	if graph.ChildrenTotal > graph.ChildrenShown {
		fmt.Printf("  children: shown %d of %d\n", graph.ChildrenShown, graph.ChildrenTotal)
	}
	if len(graph.Limits) > 0 {
		fmt.Printf("  limits: %s\n", strings.Join(graph.Limits, ", "))
	}
	for _, warning := range result.Warnings {
		fmt.Printf("  warning: %s\n", warning)
	}
}

func printIssueOpsViewText(result issueopsviewcontract.Result) {
	timeline := result.Timeline
	if timeline == nil {
		timeline = &issueopsviewcontract.CycleTimeline{}
	}
	fmt.Printf("%s: shown %d of %d (undated %d) source=%s\n",
		result.View, timeline.EventsShown, timeline.EventsTotal, timeline.Undated, result.Source)
	if result.Source == issueopsviewcontract.SourceRetained {
		fmt.Printf("  outcome=%s retained_at=%s\n", viewTextField(result.Outcome), viewTextField(result.RetainedAt))
	}
	if len(timeline.Limits) > 0 {
		fmt.Printf("  limits: %s\n", strings.Join(timeline.Limits, ", "))
	}
	for _, event := range timeline.Events {
		fmt.Printf("  %s  %s  %s  %s  %s  %s\n", event.At, event.Kind,
			viewTextField(event.Phase), viewTextField(event.Subject), viewTextField(event.Ref), viewTextField(event.Verdict))
	}
	for _, warning := range result.Warnings {
		fmt.Printf("  warning: %s\n", warning)
	}
}

// viewTextField는 빈 칸을 "-"로 채워 열 위치가 밀리지 않게 한다.
func viewTextField(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
