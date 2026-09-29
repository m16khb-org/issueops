package augmentplan

import (
	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	docsapp "issueops/internal/application/docs"
	"time"
)

// production wiring과 같은 문서 reader를 설치한다. 이 package의 테스트는 다른
// package를 거쳐 문서 조회에 닿으므로 간접 의존까지 함께 채운다. fitness graph는
// test import를 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	Repository = augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}
	DocsIndex = (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index

}
