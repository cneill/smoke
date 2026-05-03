package smoke

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/cneill/smoke/pkg/tools/handlers"
)

func (s *Smoke) NewToolManager(ctx context.Context, mode modes.Mode) (*tools.Manager, error) {
	initializers := s.ModeToolInitializers(mode)

	toolOpts := &tools.ManagerOpts{
		ProjectPath:      s.projectPath,
		SessionName:      s.mainSessionName,
		ToolInitializers: initializers,
		PlanManager:      s.planManager,
		SkillCatalog:     s.skillCatalog,
		ElicitManager:    s.elicitManager,
	}

	toolManager, err := tools.NewManager(toolOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools manager for main smoke session: %w", err)
	}

	if len(s.mcpClients) > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		mcpTools, err := s.GetMCPTools(timeoutCtx, mode, s.mcpClients...)
		if err != nil {
			slog.Error("failed to get MCP tools", "error", err)
		}

		toolManager.AddTools(mcpTools...)
	}

	if s.teaEmitter != nil {
		toolManager.SetTeaEmitter(s.teaEmitter)
	}

	return toolManager, nil
}

func (s *Smoke) ModeToolInitializers(mode modes.Mode) []tools.Initializer {
	var enabledTools []tools.Initializer

	switch mode {
	case modes.ModeWork:
		enabledTools = handlers.WorkTools()
	case modes.ModePlanning:
		enabledTools = handlers.PlanningTools()
	case modes.ModeReview:
		enabledTools = handlers.ReviewTools()
	case modes.ModeRanking:
		enabledTools = handlers.RankingTools()
	case modes.ModeSummarize:
		enabledTools = handlers.SummarizeTools()
	default:
		enabledTools = handlers.WorkTools()
	}

	return enabledTools
}
