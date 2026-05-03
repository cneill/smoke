package smoke

import (
	"context"
	"errors"
	"fmt"

	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/tools"
)

func (s *Smoke) GetMCPTools(ctx context.Context, mode modes.Mode, clients ...*mcp.CommandClient) (tools.Tools, error) {
	results := tools.Tools{}

	for _, mcpClient := range clients {
		var (
			mcpTools tools.Tools
			err      error
		)

		// TODO: handle other modes?
		switch mode {
		case modes.ModePlanning, modes.ModeReview:
			mcpTools, err = mcpClient.PlanTools(ctx)
		case modes.ModeWork:
			mcpTools, err = mcpClient.Tools(ctx)
		}

		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, fmt.Errorf("context cancelled waiting for tools from MCP client %q: %w", mcpClient.Name(), err)
			}

			return nil, fmt.Errorf("error retrieving tools from MCP client %q: %w", mcpClient.Name(), err)
		}

		results = append(results, mcpTools...)
	}

	return results, nil
}
