package issueopsapp

import (
	"issueops/cmd/issueops/mcpcli"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/inspect"
	"issueops/internal/adapter/verification/probe/qagate"
)

// configureDocsReaders는 문서 색인·목록·heading 읽기를 설치한다.
//
// 문서를 어떻게 훑는지는 하나의 구현이고, 그 선택은 composition root의 결정이다.
// 아직 전역 reader를 사용하는 소비자의 초기화를 이곳에 모은다.
func configureDocsReaders() {
	mcpcli.DocsIndex = docs.DocsIndex
	qagate.ListDocs = docs.ListDocs
	inspect.ListDocs = docs.ListDocs
}
