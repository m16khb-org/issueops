package issueopsapp

import (
	"issueops/cmd/issueops/statecli"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	stateapp "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
	"os"
)

func stateDependencies() statecli.Dependencies {
	return newStateDependencies(statestore.StateDir(), os.Getenv("ISSUEOPS_WORKER_DIR"))
}

func newStateDependencies(dir, workerOverride string) statecli.Dependencies {
	service := newStateService(dir)
	stores := statestore.NewMaintenanceStores(dir, workerOverride)
	maintenance := stateapp.NewMaintenanceService(stateapp.MaintenanceDependencies{
		AllRoots: stores.Roots, StoreExists: stores.Exists, MaintainStore: stores.Maintain,
	})
	return statecli.Dependencies{
		Write: service.Write, Read: service.Read, List: service.List, Prune: service.Prune,
		Doctor:   func() (statecontract.StateDoctorResult, error) { return statestore.Doctor(dir) },
		Maintain: maintenance.Maintain,
	}
}

func newStateService(dir string) *stateapp.Service {
	return stateapp.NewService(stateapp.Dependencies{
		StateDir: func() string { return dir }, StatePath: statepath.Path,
		OpenStore:       func(dir string) (stateport.Store, error) { return sqlstore.Open(dir) },
		ExistingRecords: statestore.ExistingRecords{},
	})
}
