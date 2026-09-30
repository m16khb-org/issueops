package basiccli

import (
	"flag"
	"fmt"
)

func (command Command) RunDocs(args []string) error {
	if len(args) > 0 && args[0] == "index" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if command.DocsIndex == nil {
		return fmt.Errorf("docs index reader is not configured")
	}
	result := command.DocsIndex(command.IssueOpsRoot, command.Version)
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
	return nil
}
