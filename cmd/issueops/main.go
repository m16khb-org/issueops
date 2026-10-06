package main

import (
	"os"

	"issueops/cmd/issueops/issueopsapp"
)

var osExit = os.Exit

func main() {
	osExit(issueopsapp.RunRootCommand(os.Args[1:]))
}
