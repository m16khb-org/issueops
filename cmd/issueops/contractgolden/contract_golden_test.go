package contractgolden

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	cliadapter "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
)

const version = "0.1.0"

var updateGolden = flag.Bool("update", false, "update golden files")

func TestCLIUsageGolden(t *testing.T) {
	assertGolden(t, "usage.golden.txt", []byte(cliadapter.Usage(version)))
}

func TestMCPToolsGolden(t *testing.T) {
	assertJSONGolden(t, "mcp_tools.golden.json", mcpcatalog.Build().Tools)
}

func TestMCPResourcesGolden(t *testing.T) {
	assertJSONGolden(t, "mcp_resources.golden.json", mcpcatalog.Build().Resources)
}

func assertJSONGolden(t *testing.T, name string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, name, append(b, '\n'))
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./cmd/issueops/contractgolden -run Golden -update)", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
	}
}
