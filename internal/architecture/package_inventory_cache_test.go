package architecture

import (
	"errors"
	"reflect"
	"testing"
)

func assertPackageInventoryCacheUsesOneSharedReadAndOneFreshStabilityRead(t *testing.T) {
	t.Helper()
	calls := 0
	run := func(string) ([]byte, error) {
		calls++
		return packageInventoryFixture(), nil
	}
	cache := newPackageInventoryCache(run)

	// The suite has 13 edge views in total; the stability read below is the
	// one edge view that deliberately bypasses the shared inventory.
	for range 12 {
		if _, err := cache.productionEdges("/repo"); err != nil {
			t.Fatal(err)
		}
	}
	for range 10 {
		if _, err := cache.productionPackages("/repo"); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if _, err := cache.modulePackages("/repo"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("shared inventory reads = %d, want 1", calls)
	}

	if _, err := loadFreshProductionEdgesFrom("/repo", run); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("total inventory reads with stability check = %d, want 2", calls)
	}
}

func assertPackageInventoryCacheReturnsDefensiveViews(t *testing.T) {
	t.Helper()
	cache := newPackageInventoryCache(func(string) ([]byte, error) {
		return packageInventoryFixture(), nil
	})

	edges, err := cache.productionEdges("/repo")
	if err != nil {
		t.Fatal(err)
	}
	edges[0] = dependencyEdge{importer: "mutated", imported: "mutated"}
	gotEdges, err := cache.productionEdges("/repo")
	if err != nil {
		t.Fatal(err)
	}
	wantEdges := []dependencyEdge{
		{importer: "internal/application/example", imported: "internal/domain/example"},
		{importer: "internal/domain/example", imported: "fmt"},
	}
	if !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Fatalf("production edges changed through caller mutation: got %v want %v", gotEdges, wantEdges)
	}

	packages, err := cache.productionPackages("/repo")
	if err != nil {
		t.Fatal(err)
	}
	packages[0] = "mutated"
	gotPackages, err := cache.productionPackages("/repo")
	if err != nil {
		t.Fatal(err)
	}
	wantPackages := []string{"internal/application/example", "internal/domain/example"}
	if !reflect.DeepEqual(gotPackages, wantPackages) {
		t.Fatalf("production packages changed through caller mutation: got %v want %v", gotPackages, wantPackages)
	}

	modules, err := cache.modulePackages("/repo")
	if err != nil {
		t.Fatal(err)
	}
	modules[0].ImportPath = "mutated"
	modules[0].GoFiles[0] = "mutated.go"
	modules[0].Imports[0] = "mutated/import"
	modules[0].TestImports[0] = "mutated/test"
	modules[0].XTestImports[0] = "mutated/xtest"
	modules = append(modules, modulePackage{ImportPath: "mutated/extra"})
	gotModules, err := cache.modulePackages("/repo")
	if err != nil {
		t.Fatal(err)
	}
	wantModules := []modulePackage{
		{
			ImportPath:   "issueops/internal/application/example",
			Name:         "example",
			GoFiles:      []string{"service.go"},
			Imports:      []string{"issueops/internal/domain/example"},
			TestImports:  []string{"issueops/internal/testsupport"},
			XTestImports: []string{"issueops/internal/externaltest"},
		},
		{
			ImportPath: "issueops/internal/domain/example",
			Name:       "example",
			GoFiles:    []string{"rule.go"},
			Imports:    []string{"fmt"},
		},
	}
	if !reflect.DeepEqual(gotModules, wantModules) {
		t.Fatalf("module packages changed through caller mutation: got %#v want %#v", gotModules, wantModules)
	}
}

func assertPackageInventorySeparatesProductionAndTestImports(t *testing.T) {
	t.Helper()
	cache := newPackageInventoryCache(func(string) ([]byte, error) {
		return packageInventoryFixture(), nil
	})

	edges, err := cache.productionEdges("/repo")
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range edges {
		if edge.imported == "internal/testsupport" || edge.imported == "internal/externaltest" {
			t.Fatalf("test-only import leaked into production edge: %v", edge)
		}
	}
	modules, err := cache.modulePackages("/repo")
	if err != nil {
		t.Fatal(err)
	}
	wantImports := []string{
		"issueops/internal/domain/example",
		"issueops/internal/testsupport",
		"issueops/internal/externaltest",
	}
	if got := modules[0].allImports(); !reflect.DeepEqual(got, wantImports) {
		t.Fatalf("module imports = %v, want %v", got, wantImports)
	}
}

func assertPackageInventoryPropagatesCommandFailure(t *testing.T) {
	t.Helper()
	want := errors.New("exit status 42")
	cache := newPackageInventoryCache(func(string) ([]byte, error) {
		return nil, want
	})

	_, err := cache.productionEdges("/repo")
	if !errors.Is(err, want) {
		t.Fatalf("command error = %v, want wrapped sentinel", err)
	}
	if got := err.Error(); got != "go list -json ./...: exit status 42" {
		t.Fatalf("command error = %q", got)
	}
}

func assertPackageInventoryRejectsTruncatedJSON(t *testing.T) {
	t.Helper()
	cache := newPackageInventoryCache(func(string) ([]byte, error) {
		return []byte(`{"ImportPath":"issueops/internal/domain/example","Imports":["fmt"]`), nil
	})

	_, err := cache.productionPackages("/repo")
	if err == nil {
		t.Fatal("truncated go list JSON unexpectedly succeeded")
	}
	if got := err.Error(); got != "decode go list package: unexpected EOF" {
		t.Fatalf("decode error = %q", got)
	}
}

func packageInventoryFixture() []byte {
	return []byte(`
{"ImportPath":"issueops/internal/application/example","Name":"example","GoFiles":["service.go"],"Imports":["issueops/internal/domain/example"],"TestImports":["issueops/internal/testsupport"],"XTestImports":["issueops/internal/externaltest"]}
{"ImportPath":"issueops/internal/domain/example","Name":"example","GoFiles":["rule.go"],"Imports":["fmt"]}
`)
}
