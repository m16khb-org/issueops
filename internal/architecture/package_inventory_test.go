package architecture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"testing"
)

const modulePrefix = "issueops/"

type modulePackage struct {
	ImportPath   string
	Name         string
	GoFiles      []string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func (p modulePackage) allImports() []string {
	all := make([]string, 0, len(p.Imports)+len(p.TestImports)+len(p.XTestImports))
	all = append(all, p.Imports...)
	all = append(all, p.TestImports...)
	all = append(all, p.XTestImports...)
	return all
}

type packageListCommand func(repoRoot string) ([]byte, error)

type packageInventoryCache struct {
	once      sync.Once
	run       packageListCommand
	packages  []modulePackage
	loadError error
}

func newPackageInventoryCache(run packageListCommand) *packageInventoryCache {
	return &packageInventoryCache{run: run}
}

func (c *packageInventoryCache) load(repoRoot string) ([]modulePackage, error) {
	c.once.Do(func() {
		c.packages, c.loadError = readPackageInventory(repoRoot, c.run)
	})
	return c.packages, c.loadError
}

func (c *packageInventoryCache) productionEdges(repoRoot string) ([]dependencyEdge, error) {
	packages, err := c.load(repoRoot)
	if err != nil {
		return nil, err
	}
	return productionEdgesFrom(packages), nil
}

func (c *packageInventoryCache) productionPackages(repoRoot string) ([]string, error) {
	packages, err := c.load(repoRoot)
	if err != nil {
		return nil, err
	}
	return productionPackagesFrom(packages), nil
}

func (c *packageInventoryCache) modulePackages(repoRoot string) ([]modulePackage, error) {
	packages, err := c.load(repoRoot)
	if err != nil {
		return nil, err
	}
	return cloneModulePackages(packages), nil
}

func readPackageInventory(repoRoot string, run packageListCommand) ([]modulePackage, error) {
	output, err := run(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("go list -json ./...: %w", err)
	}
	return decodePackageInventory(output)
}

func decodePackageInventory(output []byte) ([]modulePackage, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	var packages []modulePackage
	for {
		var pkg modulePackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode go list package: %w", err)
		}
		if strings.HasPrefix(pkg.ImportPath, modulePrefix) {
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

func runPackageList(repoRoot string) ([]byte, error) {
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = repoRoot
	return command.Output()
}

func productionEdgesFrom(packages []modulePackage) []dependencyEdge {
	var edges []dependencyEdge
	for _, pkg := range packages {
		for _, imported := range pkg.Imports {
			edges = append(edges, dependencyEdge{normalizeImport(pkg.ImportPath), normalizeImport(imported)})
		}
	}
	return sortedEdges(edges)
}

func productionPackagesFrom(inventory []modulePackage) []string {
	packages := make([]string, 0, len(inventory))
	for _, pkg := range inventory {
		packages = append(packages, normalizeImport(pkg.ImportPath))
	}
	sort.Strings(packages)
	return packages
}

func cloneModulePackages(packages []modulePackage) []modulePackage {
	cloned := make([]modulePackage, len(packages))
	for index, pkg := range packages {
		pkg.GoFiles = append([]string(nil), pkg.GoFiles...)
		pkg.Imports = append([]string(nil), pkg.Imports...)
		pkg.TestImports = append([]string(nil), pkg.TestImports...)
		pkg.XTestImports = append([]string(nil), pkg.XTestImports...)
		cloned[index] = pkg
	}
	return cloned
}

var sharedPackageInventory = newPackageInventoryCache(runPackageList)

func loadProductionEdges(t *testing.T) []dependencyEdge {
	t.Helper()
	edges, err := sharedPackageInventory.productionEdges(findRepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return edges
}

func loadFreshProductionEdges(t *testing.T) []dependencyEdge {
	t.Helper()
	edges, err := loadFreshProductionEdgesFrom(findRepoRoot(t), runPackageList)
	if err != nil {
		t.Fatal(err)
	}
	return edges
}

func loadFreshProductionEdgesFrom(repoRoot string, run packageListCommand) ([]dependencyEdge, error) {
	packages, err := readPackageInventory(repoRoot, run)
	if err != nil {
		return nil, err
	}
	return productionEdgesFrom(packages), nil
}

func loadProductionPackages(t *testing.T) []string {
	t.Helper()
	packages, err := sharedPackageInventory.productionPackages(findRepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return packages
}

func loadModulePackages(t *testing.T) []modulePackage {
	t.Helper()
	packages, err := sharedPackageInventory.modulePackages(findRepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return packages
}
