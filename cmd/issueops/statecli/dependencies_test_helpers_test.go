package statecli

import (
	statestore "issueops/internal/adapter/outbound/state"
	stateapp "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"os"
)

// testDependencies는 실제 state store를 조립한다. fitness graph는 test import를
// 수집하지 않으므로, CLI 동작은 production wiring과 같은 구현으로 검증한다.
func testDependencies() Dependencies {
	stores := statestore.NewMaintenanceStores(statestore.StateDir(), os.Getenv("ISSUEOPS_WORKER_DIR"))
	maintenance := stateapp.NewMaintenanceService(stateapp.MaintenanceDependencies{AllRoots: stores.Roots, StoreExists: stores.Exists, MaintainStore: stores.Maintain})
	service := statestore.NewService()
	return Dependencies{
		Write:    service.Write,
		Read:     service.Read,
		List:     service.List,
		Prune:    service.Prune,
		Doctor:   func() (statecontract.StateDoctorResult, error) { return statestore.Doctor(statestore.StateDir()) },
		Maintain: maintenance.Maintain,
	}
}
