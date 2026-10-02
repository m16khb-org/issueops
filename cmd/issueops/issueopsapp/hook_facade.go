package issueopsapp

import (
	"issueops/cmd/issueops/hookcli"
	"issueops/cmd/issueops/hookcli/hookcatalog"
	renderer "issueops/internal/adapter/hookprompt"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/projectdoc"
	app "issueops/internal/application/hookprompt"
)

func newHookConfig() hookcatalog.Config {
	target := resolveTarget("")
	service := app.CatalogService{
		DiscoverReport:        projectdoc.DiscoverProjectDocsReport,
		FormatCompact:         projectdoc.FormatProjectDocCatalog,
		FormatUserView:        renderer.RenderProjectDocCatalogUserView,
		FormatCompactOmission: projectdoc.FormatProjectDocCatalogOmissions,
		FormatUserOmission:    renderer.RenderProjectDocCatalogOmissions,
	}
	return hookcatalog.Config{
		ResolveTarget: func(string) string { return target },
		BuildCatalog:  service.Build,
		PrintJSON:     printJSON,
		FormatContext: hostprotocol.FormatHookContext,
	}
}

func runHook(args []string) error {
	return hookcli.RunHook(args, newHookConfig())
}
