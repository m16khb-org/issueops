package state

import (
	statecontract "issueops/internal/contract/state"
)

// MaintenanceDependencies are the store-facing hooks MaintenanceService needs:
// enumerate candidate roots, test whether a store exists under a root, and
// run maintenance on one store.
type MaintenanceDependencies struct {
	AllRoots      func() ([]string, error)
	StoreExists   func(root string) bool
	MaintainStore func(root string) (statecontract.StoreMaintainResult, error)
}

// MaintenanceService runs store maintenance across every known state root,
// skipping roots that have no store yet.
type MaintenanceService struct {
	dependencies MaintenanceDependencies
}

// NewMaintenanceService returns a MaintenanceService backed by dependencies.
func NewMaintenanceService(dependencies MaintenanceDependencies) *MaintenanceService {
	return &MaintenanceService{dependencies: dependencies}
}

// Maintain runs maintenance on each root that has an existing store and lists
// the rest under Skipped. It stops at the first root enumeration or
// maintenance error and returns the partial result with OK unset.
func (service *MaintenanceService) Maintain() (statecontract.MaintainResult, error) {
	result := statecontract.MaintainResult{Roots: []statecontract.StoreMaintainResult{}}
	roots, err := service.dependencies.AllRoots()
	if err != nil {
		return result, err
	}
	for _, root := range roots {
		if !service.dependencies.StoreExists(root) {
			result.Skipped = append(result.Skipped, root)
			continue
		}
		maintained, err := service.dependencies.MaintainStore(root)
		if err != nil {
			return result, err
		}
		result.Roots = append(result.Roots, maintained)
	}
	result.OK = true
	return result, nil
}
