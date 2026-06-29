package smoke

import (
	"fmt"

	"github.com/cneill/smoke/pkg/plan"
)

func (s *Smoke) StartNewPlan() error {
	if s.planStore == nil {
		store, err := plan.NewStore(s.projectPath)
		if err != nil {
			return fmt.Errorf("failed to initialize plan store: %w", err)
		}

		s.planStore = store
	}

	manager, metadata, err := s.planStore.NewLazyManager(s.mainSessionName)
	if err != nil {
		return fmt.Errorf("failed to create fresh plan manager: %w", err)
	}

	s.setActivePlan(manager, metadata)

	return nil
}

func (s *Smoke) ResumePlan(planID string) (plan.Metadata, error) {
	if s.planStore == nil {
		return plan.Metadata{}, fmt.Errorf("plan store not initialized")
	}

	manager, metadata, err := s.planStore.Open(planID)
	if err != nil {
		return plan.Metadata{}, fmt.Errorf("failed to resume plan %q: %w", planID, err)
	}

	s.setActivePlan(manager, metadata)

	return metadata, nil
}

func (s *Smoke) ListPlans() ([]plan.Metadata, error) {
	if s.planStore == nil {
		return nil, fmt.Errorf("plan store not initialized")
	}

	plans, err := s.planStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}

	return plans, nil
}

func (s *Smoke) CurrentPlan() plan.Metadata {
	return s.activePlan
}

func (s *Smoke) setActivePlan(manager *plan.Manager, metadata plan.Metadata) {
	s.planManager = manager
	s.activePlan = metadata

	if session := s.getMainSession(); session != nil && session.Tools != nil {
		session.Tools.SetPlanManager(manager)
	}
}
