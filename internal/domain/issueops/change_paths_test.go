package issueops

import (
	"reflect"
	"testing"
)

// 스키마 경로 규칙은 schema-evidence 게이트와 리뷰 티어가 함께 쓰는 판정이다.
// 어댑터에서 도메인으로 옮겨도 인정 범위는 그대로여야 한다.
func TestPathIsSchemaChangeAcceptsOnlyCertainSignals(t *testing.T) {
	for _, path := range []string{
		"db/001_init.sql", "prisma/schema.prisma", "src/user.entity.ts",
		"internal/user.entity.go", "app/entities/user.rb", "db/migrations/x.rb", "db/migration/y.rb",
	} {
		if !PathIsSchemaChange(path) {
			t.Fatalf("%s must be a schema change", path)
		}
	}
	for _, path := range []string{
		"", "README.md", "internal/adapter/user.go", "docs/migrations.md", "src/entitylist.ts",
	} {
		if PathIsSchemaChange(path) {
			t.Fatalf("%s must not be a schema change", path)
		}
	}
}

// 어댑터에서 옮겨 오며 경로 정규화에 `\\` -> `/` 치환이 붙었다. git은 항상 `/`를
// 내보내므로 실제 입력 집합은 옛 규칙과 같고, Windows 구분자에서만 더 넓다.
// 우연이 아니라 의도임을 여기서 고정한다.
func TestPathIsSchemaChangeAlsoAcceptsWindowsSeparators(t *testing.T) {
	if !PathIsSchemaChange(`db\migrations\001.rb`) {
		t.Fatal("a backslash-separated migrations path must classify like its slash form")
	}
	if !PathIsSchemaChange("db/migrations/001.rb") {
		t.Fatal("the slash form must keep classifying")
	}
	if got := ClassifyChangeTier([]string{`db\migrations\001.rb`}); got != ChangeTierSchemaAuth {
		t.Fatalf("tier = %q, want schema-auth", got)
	}
}

func TestClassifyChangeTierRanksSchemaAuthAboveContractAndDocs(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		paths []string
		want  ChangeTier
	}{
		{name: "empty change set", paths: nil, want: ChangeTierDefault},
		{
			name:  "docs only",
			paths: []string{".issueops/ADR.md", "README.md", "skills/x/SKILL.md", "docs/assets/note.md"},
			want:  ChangeTierDocsOnly,
		},
		{
			name:  "contract surface",
			paths: []string{"internal/contract/issueops/types.go", "internal/adapter/x.go"},
			want:  ChangeTierContract,
		},
		{
			name:  "usage golden is contract surface",
			paths: []string{"cmd/issueops/testdata/usage.golden.txt"},
			want:  ChangeTierContract,
		},
		{
			name:  "schema outranks contract",
			paths: []string{"internal/contract/issueops/types.go", "db/migrations/001.sql"},
			want:  ChangeTierSchemaAuth,
		},
		{
			name:  "auth directory outranks docs",
			paths: []string{".issueops/CAUTIONS.md", "internal/auth/token.go"},
			want:  ChangeTierSchemaAuth,
		},
		{
			name:  "plain code",
			paths: []string{"internal/adapter/issueops/x.go"},
			want:  ChangeTierDefault,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ClassifyChangeTier(testCase.paths); got != testCase.want {
				t.Fatalf("ClassifyChangeTier(%v) = %q, want %q", testCase.paths, got, testCase.want)
			}
		})
	}
}

// auth 휴리스틱은 디렉터리 세그먼트 완전일치만 인정한다. 부분 문자열 일치를
// 허용하면 오탐이 곧 최고 티어가 되어 관계없는 사이클의 리뷰 비용을 올린다.
func TestClassifyChangeTierDoesNotMatchAuthSubstrings(t *testing.T) {
	for _, path := range []string{
		"internal/author/render.go", "pkg/oauthclient/client.go",
		"internal/authoring/x.go", "internal/permissionset/x.go",
	} {
		if got := ClassifyChangeTier([]string{path}); got == ChangeTierSchemaAuth {
			t.Fatalf("%s must not be schema-auth (substring match)", path)
		}
	}
	if got := ClassifyChangeTier([]string{"docs/authoring.md"}); got != ChangeTierDocsOnly {
		t.Fatalf("docs/authoring.md = %q, want docs-only", got)
	}
	for _, path := range []string{
		"internal/auth/token.go", "api/authz/policy.go", "svc/authn/x.go",
		"app/permissions/x.go", "ops/credentials/x.go",
	} {
		if got := ClassifyChangeTier([]string{path}); got != ChangeTierSchemaAuth {
			t.Fatalf("%s = %q, want schema-auth (exact segment)", path, got)
		}
	}
}

func TestReviewLensesForTierNarrowsDocsOnlyAndLeadsWithCompat(t *testing.T) {
	if got := ReviewLensesForTier(ChangeTierDocsOnly); !reflect.DeepEqual(got, []string{"side-effect"}) {
		t.Fatalf("docs-only lenses = %v, want [side-effect]", got)
	}
	contract := ReviewLensesForTier(ChangeTierContract)
	if len(contract) != 4 || contract[0] != "compat" {
		t.Fatalf("contract lenses = %v, want four lenses led by compat", contract)
	}
	for _, tier := range []ChangeTier{ChangeTierSchemaAuth, ChangeTierDefault} {
		if got := ReviewLensesForTier(tier); len(got) != 4 {
			t.Fatalf("%s lenses = %v, want all four", tier, got)
		}
	}
	// 병렬 허용 여부는 코드가 아니라 issueops-verify 스킬 문장이 소유한다.
	for _, tier := range []ChangeTier{ChangeTierDocsOnly, ChangeTierContract, ChangeTierSchemaAuth, ChangeTierDefault} {
		for _, lens := range ReviewLensesForTier(tier) {
			if lens == "parallel-allowed" {
				t.Fatalf("%s must not carry a parallel marker", tier)
			}
		}
	}
}

// frontend 신호는 QA 라우팅 힌트다. 위험 순위(tier)와 별개이며 티어를 바꾸지 않는다.
func TestPathIsFrontendChangeMatchesExtensionsAndSegments(t *testing.T) {
	for _, path := range []string{
		"app/Home.tsx", "src/Button.jsx", "web/App.vue", "ui/Card.svelte", "site/index.astro",
		"styles/main.css", "theme/app.scss", "legacy/old.less", "public/index.html",
		"components/Button.go", "pages/api.go", "styles/tokens.json",
	} {
		if !PathIsFrontendChange(path) {
			t.Fatalf("%s must be a frontend change", path)
		}
	}
	for _, path := range []string{
		"", "README.md", "internal/adapter/user.go", "internal/componentsx/a.go",
		"pkg/pageset/b.go", "cmd/publicity/c.go",
	} {
		if PathIsFrontendChange(path) {
			t.Fatalf("%s must not be a frontend change", path)
		}
	}
}

// 알려진 오탐과 누락을 여기 고정한다. 신호는 라우팅 힌트이므로 오탐 비용은
// `Not Run` 한 줄이고 누락 비용은 QA 미제안이다.
func TestPathIsFrontendChangeKnownFalsePositivesAndNegatives(t *testing.T) {
	if !PathIsFrontendChange("skills/aside-functional-qa/testdata/client-qa-fixture.html") {
		t.Fatal("documented false positive: an .html test fixture trips the signal")
	}
	for _, missed := range []string{"app/Home.js", "src/ui/store.ts", "src/app.component.ts"} {
		if PathIsFrontendChange(missed) {
			t.Fatalf("documented false negative changed: %s now matches", missed)
		}
	}
}

func TestHasFrontendChangeIsIndependentOfTier(t *testing.T) {
	paths := []string{".issueops/ADR.md", "app/Home.tsx"}
	if !HasFrontendChange(paths) {
		t.Fatal("one frontend path is enough")
	}
	if got := ClassifyChangeTier(paths); got != ChangeTierDocsOnly && got != ChangeTierDefault {
		t.Fatalf("the frontend signal must not change the tier ranking, got %q", got)
	}
	if HasFrontendChange([]string{"internal/a.go"}) || HasFrontendChange(nil) {
		t.Fatal("no frontend path means no signal")
	}
}
