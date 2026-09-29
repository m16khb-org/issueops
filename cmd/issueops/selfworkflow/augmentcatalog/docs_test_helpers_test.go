package augmentcatalog

import (
	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
)

// production wiring과 같은 문서 reader를 설치한다. augmentplan이 이 package를
// import하므로 여기서 역방향으로 채우면 순환이 된다 — 자기 것만 설치한다.
func init() {
	repo := augmentation.Repository{ListDocs: docs.ListDocs}
	CollectSelfAugmentRepoSignals = repo.CollectSignals
	DocsContainTerm = repo.DocsContainTerm
	FileContainsTerm = augmentation.FileContainsTerm
	DirContainsTerm = augmentation.DirContainsTerm
}

func readmeContainsTerm(root, term string) bool {
	return FileContainsTerm(root, "README.md", term) || FileContainsTerm(root, "README.en.md", term)
}
