// Package issueopsartifact 패키지는 이슈 실행에 앞서 아티팩트를 스테이징하는 애플리케이션 계층의
// 유스케이스를 제공한다. 검증은 도메인에, 영속화는 Repository 포트에 위임한다.
package issueopsartifact

import (
	"context"
	"fmt"
	"sort"

	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
	issueopsartifactdomain "issueops/internal/domain/issueopsartifact"
)

// Service 타입은 이슈 실행 전에 아티팩트를 스테이징·해제·조회하는 애플리케이션 서비스다.
// 이름 정규화와 봉인(seal) 규칙은 도메인에 위임하고 저장은 Repository에 맡긴다.
type Service struct {
	repository Repository
}

// NewService 함수는 repository를 사용하는 Service를 만든다.
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// Stage 메서드는 id 레코드에 name 아티팩트의 content를 스테이징한다. 이름·본문이 유효하지 않거나
// 저장소가 없으면 오류를 돌려주고, 이미 봉인된 레코드에는 SealedError를 돌려준다.
func (service *Service) Stage(
	ctx context.Context,
	stateRoot string,
	id string,
	name string,
	content []byte,
) (issueopsartifactcontract.Record, error) {
	if service == nil || service.repository == nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops artifact repository is required",
		)
	}
	name, err := issueopsartifactdomain.NormalizeName(name)
	if err != nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, err
	}
	if err := issueopsartifactdomain.ValidateContent(content); err != nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsartifactcontract.Record,
		staged issueopsartifactcontract.Staged,
	) (issueopsartifactcontract.Staged, error) {
		if !issueopsartifactdomain.CanStage(record, name) {
			return staged, &issueopsartifactcontract.SealedError{ID: record.ID}
		}
		staged[name] = string(content)
		return staged, nil
	})
}

// Unstage 메서드는 id 레코드에서 name 아티팩트를 제거한다. execution prepare 이후에는 봉인되어
// 있으므로 오류를 돌려주며, 저장소가 없거나 이름이 유효하지 않아도 오류를 돌려준다.
func (service *Service) Unstage(
	ctx context.Context,
	stateRoot string,
	id string,
	name string,
) (issueopsartifactcontract.Record, error) {
	if service == nil || service.repository == nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, fmt.Errorf(
			"issueops artifact repository is required",
		)
	}
	name, err := issueopsartifactdomain.NormalizeName(name)
	if err != nil {
		return issueopsartifactcontract.Record{OK: false, ID: id}, err
	}
	return service.repository.Update(ctx, stateRoot, id, func(
		record issueopsartifactcontract.Record,
		staged issueopsartifactcontract.Staged,
	) (issueopsartifactcontract.Staged, error) {
		if record.Execution != nil {
			return staged, fmt.Errorf(
				"staged artifacts are sealed after execution prepare and cannot be unstaged",
			)
		}
		delete(staged, name)
		return staged, nil
	})
}

// Staged 메서드는 스테이징된 아티팩트의 이름→본문 map을 돌려준다. 호출자가 본문 digest를 한 번의
// 읽기로 계산할 수 있게 한다.
func (service *Service) Staged(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsartifactcontract.Staged, error) {
	if service == nil || service.repository == nil {
		return nil, fmt.Errorf("issueops artifact repository is required")
	}
	return service.repository.ReadStaged(ctx, stateRoot, id)
}

// Names 메서드는 id 레코드에 스테이징된 아티팩트 이름을 정렬해 돌려준다.
func (service *Service) Names(
	ctx context.Context,
	stateRoot string,
	id string,
) ([]string, error) {
	if service == nil || service.repository == nil {
		return nil, fmt.Errorf("issueops artifact repository is required")
	}
	staged, err := service.repository.ReadStaged(ctx, stateRoot, id)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(staged))
	for name := range staged {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
