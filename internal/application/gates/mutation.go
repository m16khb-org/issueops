package gates

import (
	model "issueops/internal/contract/gates"
	domain "issueops/internal/domain/gates"
)

// Init 메서드는 요청의 게이트 명세로 새 원장 파일을 만든다.
// 이미 파일이 있으면 덮어쓰지 않고 에러를 돌려준다.
func (service Service) Init(req model.InitRequest) (model.InitResult, error) {
	req, result, err := domain.PrepareInit(req)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateCreate(req.File, service.Store.ExistsFile(req.File)); err != nil {
		return result, err
	}
	body, err := domain.RenderInitialLedger(req)
	if err != nil {
		return result, err
	}
	if err := service.Store.Create(req.File, []byte(body)); err != nil {
		return result, err
	}
	return domain.CreatedLedger(result, body), nil
}

// Abandon 메서드는 원장의 한 게이트를 ABANDON으로 기록하고 갱신된 원장을 파일 모드를 유지한 채 다시 쓴다.
// 요청이 유효하지 않거나 대상 게이트가 없거나 이미 abandon된 게이트면 에러를 돌려준다.
func (service Service) Abandon(req model.AbandonRequest) (model.AbandonResult, error) {
	req, result, err := domain.PrepareAbandon(req)
	if err != nil {
		return result, err
	}
	data, err := service.Store.Read(req.File)
	if err != nil {
		return result, err
	}
	ledger := domain.Parse(string(data))
	if err := domain.AbandonLedger(&ledger, req); err != nil {
		return result, err
	}
	if err := service.Store.WritePreservingMode(req.File, []byte(domain.Render(ledger))); err != nil {
		return result, err
	}
	return domain.RecordedAbandon(result), nil
}
