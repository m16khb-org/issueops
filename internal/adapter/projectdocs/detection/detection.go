package detection

import (
	"path/filepath"
	"slices"
	"strings"
)

// frameworkConfigFiles maps config file base names to the framework they imply.
var frameworkConfigFiles = map[string]string{
	"next.config.js":          "Next.js",
	"next.config.mjs":         "Next.js",
	"next.config.ts":          "Next.js",
	"vite.config.js":          "Vite",
	"vite.config.mjs":         "Vite",
	"vite.config.ts":          "Vite",
	"nuxt.config.js":          "Nuxt",
	"nuxt.config.ts":          "Nuxt",
	"astro.config.js":         "Astro",
	"astro.config.mjs":        "Astro",
	"astro.config.ts":         "Astro",
	"nest-cli.json":           "NestJS",
	"tauri.conf.json":         "Tauri",
	"tauri.conf.json5":        "Tauri",
	"electron-builder.yml":    "Electron",
	"electron-builder.yaml":   "Electron",
	"electron.vite.config.ts": "Electron",
}

// goModuleFrameworks lists Go module path fragments in match priority order:
// the first fragment contained in a module wins.
var goModuleFrameworks = []struct{ fragment, framework string }{
	{"github.com/spf13/cobra", "Cobra"},
	{"github.com/gin-gonic/gin", "Gin"},
	{"github.com/go-chi/chi", "chi"},
	{"github.com/labstack/echo", "Echo"},
}

// packageDependencyFrameworks maps package.json dependency names to frameworks.
var packageDependencyFrameworks = map[string]string{
	"react":           "React",
	"next":            "Next.js",
	"vite":            "Vite",
	"vue":             "Vue",
	"svelte":          "Svelte",
	"@angular/core":   "Angular",
	"express":         "Express",
	"@nestjs/core":    "NestJS",
	"fastify":         "Fastify",
	"prisma":          "Prisma",
	"@prisma/client":  "Prisma",
	"electron":        "Electron",
	"@tauri-apps/api": "Tauri",
	"@tauri-apps/cli": "Tauri",
}

func Frameworks(root string, files []string, addEvidence func(string)) []string {
	frameworks := []string{}
	addFramework := func(name, evidence string) {
		frameworks = appendUnique(frameworks, name)
		if evidence != "" {
			addEvidence(evidence)
		}
	}
	for _, rel := range files {
		if name, ok := frameworkConfigFiles[filepath.Base(rel)]; ok {
			addFramework(name, rel)
		}
		if rel == "go.mod" {
			addGoModuleFrameworks(readGoModules(root), addFramework)
		}
		if rel == "package.json" {
			addPackageDependencyFrameworks(readPackageDependencies(filepath.Join(root, rel)), addFramework)
		}
	}
	return frameworks
}

func addGoModuleFrameworks(mods []string, addFramework func(name, evidence string)) {
	for _, mod := range mods {
		for _, candidate := range goModuleFrameworks {
			if strings.Contains(mod, candidate.fragment) {
				addFramework(candidate.framework, "go.mod:"+candidate.fragment)
				break
			}
		}
	}
}

func addPackageDependencyFrameworks(deps map[string]bool, addFramework func(name, evidence string)) {
	for dep := range deps {
		if name, ok := packageDependencyFrameworks[dep]; ok {
			addFramework(name, "package.json:"+dep)
		}
	}
}

func Monorepo(root string, files []string, addEvidence func(string)) bool {
	for _, rel := range files {
		switch rel {
		case "pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json":
			addEvidence(rel)
			return true
		}
		if strings.Contains(rel, "/") && (strings.HasSuffix(rel, "/package.json") || strings.HasSuffix(rel, "/go.mod") || strings.HasSuffix(rel, "/pyproject.toml") || strings.HasSuffix(rel, "/Cargo.toml")) {
			addEvidence(rel)
			return true
		}
	}
	if workspaces := readPackageWorkspaces(filepath.Join(root, "package.json")); len(workspaces) > 0 {
		addEvidence("package.json:workspaces")
		return true
	}
	return false
}

func ProjectTypes(root string, languages, frameworks []string, monorepo bool, addEvidence func(string)) []string {
	types := []string{}
	addType := func(v, evidence string) {
		types = appendUnique(types, v)
		if evidence != "" {
			addEvidence(evidence)
		}
	}
	if monorepo {
		addType("monorepo", "")
	}
	frontend := containsAnyString(frameworks, "React", "Next.js", "Vite", "Vue", "Svelte", "Angular", "Nuxt", "Astro")
	desktopClient := containsAnyString(frameworks, "Tauri", "Electron")
	backend := containsAnyString(frameworks, "Express", "NestJS", "Fastify", "Gin", "chi", "Echo") || containsAnyString(languages, "Go")
	cli := containsAnyString(frameworks, "Cobra")
	if frontend {
		addType("frontend", "")
	}
	if desktopClient {
		addType("desktop-client", "")
	}
	if backend {
		addType("backend", "")
	}
	if frontend && backend {
		addType("fullstack", "")
	}
	if cli || dirExists(filepath.Join(root, "cmd")) {
		addType("cli", "cmd/")
	}
	if len(types) == 0 && len(languages) > 0 {
		addType("library", "")
	}
	return types
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}
