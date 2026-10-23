package basiccli

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/cliflag"
)

func (command Command) RunDocs(args []string) error {
	if len(args) > 0 && args[0] == "index" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	all := fs.Bool("all", false, "also list dated history records (.issueops/issues, research, plans, archive, verified-execution, and dated adr/ and cautions/ files)")
	if err := cliflag.ParseNoArgs(fs, args); err != nil {
		return err
	}
	if command.DocsIndex == nil {
		return fmt.Errorf("docs index reader is not configured")
	}
	result := command.DocsIndex(command.IssueOpsRoot, command.Version, *all)
	if *jsonOut {
		return printJSON(result)
	}
	for _, doc := range result.Docs {
		if doc.Title == "" {
			fmt.Println(doc.RelPath)
			continue
		}
		fmt.Printf("%s — %s\n", doc.RelPath, doc.Title)
	}
	if result.HistoryOmitted > 0 {
		fmt.Printf("(%d history records omitted; pass --all to list them)\n", result.HistoryOmitted)
	}
	return nil
}
