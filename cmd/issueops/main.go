// Command issueops 명령은 IssueOps CLI의 진입점이다. 명령줄 인자를 issueopsapp.RunRootCommand에 넘기고 돌려받은 종료 코드로 프로세스를 끝낸다.
package main

import (
	"os"

	"issueops/cmd/issueops/issueopsapp"
)

var osExit = os.Exit

func main() {
	osExit(issueopsapp.RunRootCommand(os.Args[1:]))
}
