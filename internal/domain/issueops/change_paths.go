package issueops

import (
	"path"
	"strings"
)

// ChangeTier는 변경 집합의 성격이다. 리뷰 범위와 reviewer effort를 여기에
// 비례시켜 작은 변경이 큰 변경과 같은 비용을 내지 않게 한다.
type ChangeTier string

const (
	ChangeTierDocsOnly   ChangeTier = "docs-only"
	ChangeTierContract   ChangeTier = "contract"
	ChangeTierSchemaAuth ChangeTier = "schema-auth"
	ChangeTierDefault    ChangeTier = "default"
)

// authPathSegments는 최고 티어를 켜는 디렉터리 이름이다. 세그먼트 완전일치만
// 인정한다 — 부분 문자열을 허용하면 internal/author나 pkg/oauthclient 같은
// 무관한 경로가 최고 티어를 켜서 리뷰 비용만 올린다.
var authPathSegments = map[string]bool{
	"auth": true, "authz": true, "authn": true, "permissions": true, "credentials": true,
}

// contractPathPrefixes는 공개 계약 표면이다. 여기가 바뀌면 하위 호환 렌즈가
// 먼저 걸려야 한다.
var contractPathPrefixes = []string{
	"internal/contract/", "internal/domain/cli/", "internal/domain/commandparse/", "configs/",
}

// frontendExtensions는 사람이 보는 화면을 만드는 확장자다. `.js`와 `.ts`는
// 백엔드에서도 흔해 넣지 않는다 — 그래서 `.js` React나 `src/ui/**/*.ts`는
// 잡히지 않는 알려진 누락이다.
var frontendExtensions = map[string]bool{
	".tsx": true, ".jsx": true, ".vue": true, ".svelte": true, ".astro": true,
	".css": true, ".scss": true, ".less": true, ".html": true,
}

// frontendPathSegments는 화면 자산이 모이는 디렉터리다. 세그먼트 완전일치만
// 인정해 `componentsx`·`pageset` 같은 이름이 걸리지 않게 한다.
var frontendPathSegments = map[string]bool{
	"components": true, "pages": true, "public": true, "styles": true,
}

// docsPathPrefixes는 운영 문서와 스킬이다.
var docsPathPrefixes = []string{".issueops/", "skills/", "docs/"}

// PathIsSchemaChange는 확실한 스키마 신호만 인정한다. 오탐이 나면 DB 없는
// 사이클까지 게이트가 켜지므로, 판단이 갈리는 패턴은 일부러 뺀다.
func PathIsSchemaChange(rel string) bool {
	rel = normalizeChangePath(rel)
	if rel == "" {
		return false
	}
	base := path.Base(rel)
	if strings.HasSuffix(base, ".sql") || base == "schema.prisma" {
		return true
	}
	if strings.HasSuffix(base, ".entity.ts") || strings.HasSuffix(base, ".entity.js") || strings.HasSuffix(base, ".entity.go") {
		return true
	}
	for _, segment := range strings.Split(path.Dir(rel), "/") {
		switch segment {
		case "migrations", "migration", "entities":
			return true
		}
	}
	return false
}

// PathIsFrontendChange는 그 경로가 사람이 보는 화면을 바꾸는지 본다. 티어와
// 독립인 QA 라우팅 힌트이며 위험 순위를 바꾸지 않는다. `.html` 테스트 픽스처처럼
// 화면이 아닌 파일도 잡히는 오탐이 있는데, 오탐의 비용은 QA를 `Not Run`으로
// 적는 한 줄이라 좁게 잡아 누락을 늘리는 쪽보다 낫다.
func PathIsFrontendChange(rel string) bool {
	rel = normalizeChangePath(rel)
	if rel == "" {
		return false
	}
	if frontendExtensions[path.Ext(rel)] {
		return true
	}
	for _, segment := range strings.Split(path.Dir(rel), "/") {
		if frontendPathSegments[segment] {
			return true
		}
	}
	return false
}

// HasFrontendChange는 변경 집합에 화면 변경이 하나라도 있는지 본다.
func HasFrontendChange(paths []string) bool {
	for _, rel := range paths {
		if PathIsFrontendChange(rel) {
			return true
		}
	}
	return false
}

// ClassifyChangeTier는 변경 집합 하나를 티어로 바꾼다. 우선순위는
// schema-auth > contract > docs-only > default다. 위험이 높은 신호가 하나라도
// 있으면 그 티어가 이기고, docs-only는 전부가 문서일 때만 성립한다.
func ClassifyChangeTier(paths []string) ChangeTier {
	if len(paths) == 0 {
		return ChangeTierDefault
	}
	anyContract, allDocs := false, true
	for _, raw := range paths {
		rel := normalizeChangePath(raw)
		if rel == "" {
			continue
		}
		if PathIsSchemaChange(rel) || pathTouchesAuth(rel) {
			return ChangeTierSchemaAuth
		}
		if pathIsContractSurface(rel) {
			anyContract = true
		}
		if !pathIsDocument(rel) {
			allDocs = false
		}
	}
	switch {
	case anyContract:
		return ChangeTierContract
	case allDocs:
		return ChangeTierDocsOnly
	default:
		return ChangeTierDefault
	}
}

// ReviewLensesForTier는 그 티어에서 적용할 코드베이스 존중 렌즈를 돌려준다.
// 병렬 리뷰 허용 여부는 여기 담지 않는다 — 그 판단은 issueops-verify 스킬
// 문장이 단독으로 소유한다.
func ReviewLensesForTier(tier ChangeTier) []string {
	switch tier {
	case ChangeTierDocsOnly:
		return []string{"side-effect"}
	case ChangeTierContract:
		return []string{"compat", "reuse", "perf", "side-effect"}
	default:
		return []string{"reuse", "perf", "compat", "side-effect"}
	}
}

func normalizeChangePath(rel string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/")))
}

func pathTouchesAuth(rel string) bool {
	for _, segment := range strings.Split(path.Dir(rel), "/") {
		if authPathSegments[segment] {
			return true
		}
	}
	return false
}

func pathIsContractSurface(rel string) bool {
	base := path.Base(rel)
	if strings.Contains(base, ".golden") || strings.HasSuffix(base, ".proto") {
		return true
	}
	if strings.HasPrefix(base, "openapi") || strings.HasPrefix(base, "swagger") {
		return true
	}
	for _, prefix := range contractPathPrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func pathIsDocument(rel string) bool {
	base := path.Base(rel)
	if strings.HasSuffix(base, ".md") || strings.HasPrefix(base, "readme") {
		return true
	}
	for _, prefix := range docsPathPrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}
