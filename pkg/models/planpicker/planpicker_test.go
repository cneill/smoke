package planpicker_test

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cneill/smoke/pkg/models/planpicker"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickerSelectAndCancel(t *testing.T) {
	t.Parallel()

	plans := []plan.Metadata{
		{PlanID: "first", SessionName: "main", LastUsedAt: time.Now()},
		{PlanID: "second", SessionName: "main", LastUsedAt: time.Now().Add(-time.Minute)},
	}

	model := planpicker.New(plans, 80, 20)
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	selected, ok := cmd().(planpicker.SelectedMessage)
	require.True(t, ok)
	assert.Equal(t, "second", selected.Plan.PlanID)

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	assert.IsType(t, planpicker.CanceledMessage{}, cmd())
}

func TestPickerViewFitsHeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		numPlans int
		height   int
	}{
		{name: "few_plans_tall_screen", numPlans: 3, height: 40},
		{name: "many_plans_tall_screen", numPlans: 30, height: 40},
		{name: "many_plans_short_screen", numPlans: 30, height: 15},
		{name: "plans_equal_line_budget", numPlans: 20, height: 30},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			plans := make([]plan.Metadata, test.numPlans)
			for i := range plans {
				plans[i] = plan.Metadata{
					PlanID:      fmt.Sprintf("plan-%02d", i),
					SessionName: "main",
					LastUsedAt:  time.Now(),
					LogPath:     fmt.Sprintf("/tmp/plans/plan-%02d.jsonl", i),
				}
			}

			model := planpicker.New(plans, 120, test.height)

			// Walk the selection through every plan, checking the view never overflows and always shows the selection.
			for i := range plans {
				view := model.View()
				assert.LessOrEqual(t, lipgloss.Height(view), test.height, "selected index %d", i)
				assert.Contains(t, view, plans[i].PlanID, "selected index %d", i)

				model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
			}
		})
	}
}
