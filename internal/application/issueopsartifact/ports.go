package issueopsartifact

import (
	"context"

	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
)

// Repository 타입은 스테이징된 아티팩트를 저장하는 포트다. Update는 레코드와 현재 스테이징 맵을
// 변경 함수에 넘겨 결과를 저장하고 갱신된 레코드를 돌려주며, ReadStaged는 스테이징 맵만 읽는다.
type Repository interface {
	Update(
		context.Context,
		string,
		string,
		func(
			issueopsartifactcontract.Record,
			issueopsartifactcontract.Staged,
		) (issueopsartifactcontract.Staged, error),
	) (issueopsartifactcontract.Record, error)
	ReadStaged(context.Context, string, string) (issueopsartifactcontract.Staged, error)
}
