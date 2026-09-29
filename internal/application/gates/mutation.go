package gates

import (
	model "issueops/internal/contract/gates"
	domain "issueops/internal/domain/gates"
)

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
