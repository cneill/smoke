package tools_test

import (
	"context"
	"testing"

	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type planAwareTool struct {
	manager *plan.Manager
}

func (p *planAwareTool) Name() string             { return "plan_aware" }
func (p *planAwareTool) Description() string      { return "plan-aware test tool" }
func (p *planAwareTool) Examples() tools.Examples { return nil }
func (p *planAwareTool) Params() tools.Params     { return tools.Params{} }
func (p *planAwareTool) Run(context.Context, tools.Args) (*tools.Output, error) {
	return &tools.Output{Text: "ok"}, nil
}
func (p *planAwareTool) SetPlanManager(manager *plan.Manager) { p.manager = manager }

func TestSetPlanManagerReinjectsExistingTools(t *testing.T) {
	t.Parallel()

	initial := plan.NewManager(nil)
	next := plan.NewManager(nil)
	tool := &planAwareTool{}

	manager, err := tools.NewManager(&tools.ManagerOpts{
		ProjectPath: ".",
		SessionName: "test",
		PlanManager: initial,
	})
	require.NoError(t, err)

	manager.SetTools(tool)
	manager.SetPlanManager(next)

	assert.Same(t, next, tool.manager)
}
