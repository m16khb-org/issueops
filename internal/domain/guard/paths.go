package guard

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var secretPathRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)
})

// SecretLikePath reports whether rel looks like a secret or credential file, such as .env, private keys,
// certificates, or names containing credentials or secret.
func SecretLikePath(rel string) bool { return secretPathRe().MatchString(filepath.ToSlash(rel)) }

// RelevantPath reports whether the guard should read rel: secret-like paths, testdata, Dockerfiles, and
// files with a known source, config, or documentation extension.
func RelevantPath(rel string) bool {
	if SecretLikePath(rel) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(rel))
	if ext == "" {
		return strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.HasSuffix(rel, "Dockerfile")
	}
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh", ".bash", ".zsh", ".fish", ".yaml", ".yml", ".json", ".toml", ".md", ".sql":
		return true
	default:
		return false
	}
}

func testPath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	return strings.Contains(p, "test") || strings.Contains(p, "spec") || strings.Contains(p, "fixture") || strings.Contains(p, "golden")
}

func executableTestSourcePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	if strings.Contains(p, "testdata/") || strings.Contains(p, ".golden.") || strings.Contains(p, "/fixtures/") || strings.Contains(p, "/fixture/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh":
		return strings.Contains(p, "test") || strings.Contains(p, "spec")
	default:
		return false
	}
}

func allowsFixtureURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	return host == "example.com" ||
		host == "example.org" ||
		host == "example.net" ||
		host == "example.invalid" ||
		host == "127.0.0.1" ||
		host == "localhost" ||
		(host == "github.com" && strings.HasPrefix(path, "/example/"))
}

// SourcePath reports whether rel is production source code: it has a known source extension and is not a
// test, .issueops document, or docs file.
func SourcePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	if testPath(p) || strings.HasPrefix(p, ".issueops/") || strings.HasPrefix(p, "docs/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh", ".sql":
		return true
	default:
		return false
	}
}

func contractSurfacePath(rel string) bool {
	p := filepath.ToSlash(rel)
	return strings.HasPrefix(p, "cmd/issueops/") || strings.HasPrefix(p, "internal/adapter/") || strings.HasPrefix(p, "internal/core/")
}
