package augmentation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMCPCatalogSignalRequiresSchemaAssemblyAndRootWiring(t *testing.T) {
	root := t.TempDir()
	write := func(rel, source string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/contract/mcp/adapter_owned_catalog.go", "package mcp\nfunc AdapterOwnedTools(){}")
	write("internal/adapter/inbound/catalog/mcp/catalog.go", "package mcp\nfunc Build(){contract.AdapterOwnedTools()}")
	if hasMCPAdapterCatalog(root) {
		t.Fatal("catalog without root wiring counted as complete")
	}
	write("cmd/issueops/issueopsapp/mcp_facade.go", "package issueopsapp\nfunc issueOpsMCPDependencies() mcpcli.MCPDependencies { return mcpcli.MCPDependencies{Catalog: mcpcatalog.Build()} }")
	if !hasMCPAdapterCatalog(root) {
		t.Fatal("current schema, assembly and root wiring not observed")
	}
	if err := os.Remove(filepath.Join(root, "internal/contract/mcp/adapter_owned_catalog.go")); err != nil {
		t.Fatal(err)
	}
	write("internal/domain/mcp/adapter_owned_catalog.go", "package mcp\nfunc AdapterOwnedTools(){}")
	if hasMCPAdapterCatalog(root) {
		t.Fatal("removed domain schema owner still accepted")
	}
}

func TestCLIUsageSignalFollowsTheRendererOwner(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	repo := Repository{ListDocs: func(string) []string { return nil }}
	write("internal/adapter/inbound/catalog/cli/usage.go", "package cli\nfunc Usage(version string) string { return version }")
	if repo.CollectSignals(root, 0, nil, "").HasCLIAdapterSplit {
		t.Fatal("renderer without root wiring counted")
	}
	write("cmd/issueops/issueopsapp/app.go", "package issueopsapp\nfunc usage(){cliadapter.Usage(version)}")
	if !repo.CollectSignals(root, 0, nil, "").HasCLIAdapterSplit {
		t.Fatal("current CLI renderer and root wiring not observed")
	}
}
