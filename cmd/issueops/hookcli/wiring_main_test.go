package hookcli

import (
	"os"
	"testing"

	"encoding/json"
	"issueops/cmd/issueops/hookcli/hookcatalog"
	"issueops/cmd/issueops/pathutil"
	hookpromptadapter "issueops/internal/adapter/hookprompt"
	"issueops/internal/adapter/hostprotocol"
	projectdocadapter "issueops/internal/adapter/projectdoc"
	app "issueops/internal/application/hookprompt"
	"issueops/internal/testsupport"
)

// 프로덕션에서는 issueopsapp이 주입한다. 상속된 운영자 스위치(ISSUEOPS_DISABLE_HOOKS)
// 는 먼저 지운다 — dogfood 셸의 값이 새어 들어오면 context hook이 아무것도
// 내보내지 않아 테스트 결론이 바뀐다(#395).
func TestMain(m *testing.M) {
	testsupport.ClearInheritedOperatorSwitches()
	os.Exit(m.Run())
}

func runHook(args []string) error {
	service := app.CatalogService{DiscoverReport: projectdocadapter.DiscoverProjectDocsReport, FormatCompact: projectdocadapter.FormatProjectDocCatalog, FormatUserView: hookpromptadapter.RenderProjectDocCatalogUserView}
	return RunHook(args, hookcatalog.Config{BuildCatalog: service.Build, ResolveTarget: pathutil.ResolveTarget, FormatContext: hostprotocol.FormatHookContext, PrintJSON: func(v any) error { enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(v) }})
}
