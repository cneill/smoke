package planpicker_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
