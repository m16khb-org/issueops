package apidoc

import (
	"fmt"
	contract "issueops/internal/contract/apidoc"
)

func printAPIDocStaticCheck(result contract.StaticResult) {
	if result.OK {
		fmt.Println(result.Summary)
		return
	}
	fmt.Println(result.Summary)
	for _, v := range result.Violations {
		fmt.Printf("- %s:%d %s: %s\n", v.File, v.Line, v.Code, v.Message)
	}
}
