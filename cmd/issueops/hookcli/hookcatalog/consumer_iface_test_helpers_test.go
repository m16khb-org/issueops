package hookcatalog

import (
	renderer "issueops/internal/adapter/hookprompt"
	reader "issueops/internal/adapter/projectdoc"
	app "issueops/internal/application/hookprompt"
)

func testCatalogService() app.CatalogService {
	return app.CatalogService{DiscoverReport: reader.DiscoverProjectDocsReport, FormatCompact: reader.FormatProjectDocCatalog, FormatUserView: renderer.RenderProjectDocCatalogUserView}
}
