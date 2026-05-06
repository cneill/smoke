package smoke

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cneill/smoke/pkg/commands"
	cmdhandlers "github.com/cneill/smoke/pkg/commands/handlers"
	"github.com/cneill/smoke/pkg/elicit"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/llmctx/agentsmd"
	"github.com/cneill/smoke/pkg/llmctx/modes"
	"github.com/cneill/smoke/pkg/llmctx/skills"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/mcp"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/providers/chatgpt"
	"github.com/cneill/smoke/pkg/providers/claude"
	"github.com/cneill/smoke/pkg/providers/grok"
	"github.com/cneill/smoke/pkg/providers/ollama"
)

func (s *Smoke) setup(ctx context.Context) error {
	if err := s.OK(); err != nil {
		return fmt.Errorf("%w: %w", ErrOptions, err)
	}

	if err := s.setupPlanManager(); err != nil {
		return fmt.Errorf("failed to set up plan manager for smoke: %w", err)
	}

	s.setupAgentsmd()
	s.setupSkills()

	if err := s.setupLLM(); err != nil {
		return fmt.Errorf("failed to set up LLM: %w", err)
	}

	s.setupElicitManager()

	if err := s.setupMCPClients(ctx); err != nil {
		return fmt.Errorf("failed to set up MCP clients: %w", err)
	}

	if err := s.setupSession(ctx); err != nil {
		return fmt.Errorf("failed to set up main smoke session: %w", err)
	}

	if err := s.setupCommands(); err != nil {
		return fmt.Errorf("failed to set up commands: %w", err)
	}

	return nil
}

func (s *Smoke) setupPlanManager() error {
	// TODO: better way of setting this name...
	planFileName := s.mainSessionName + "_plan.json"

	relPath, err := fs.GetRelativePath(s.projectPath, planFileName)
	if err != nil {
		return fmt.Errorf("invalid session plan file path (%s): %w", planFileName, err)
	}

	planManager, err := plan.ManagerFromPath(relPath)
	if err != nil {
		return fmt.Errorf("failed to set up plan manager: %w", err)
	}

	s.planManager = planManager

	return nil
}

func (s *Smoke) setupAgentsmd() {
	s.agentsmdCatalog = agentsmd.Discover(s.projectPath)
}

func (s *Smoke) setupSkills() {
	s.skillCatalog = skills.Discover(s.projectPath)
}

func (s *Smoke) setupLLM() error {
	var (
		llm llms.LLM
		err error
	)

	switch s.llmConfig.Provider {
	case llms.LLMTypeChatGPT:
		llm, err = chatgpt.New(s.llmConfig)
	case llms.LLMTypeClaude:
		llm, err = claude.New(s.llmConfig)
	case llms.LLMTypeGrok:
		llm, err = grok.New(s.llmConfig)
	case llms.LLMTypeOllama:
		llm, err = ollama.New(s.llmConfig)
	default:
		err = fmt.Errorf("unknown provider: %s", s.llmConfig.Provider)
	}

	if err != nil {
		return fmt.Errorf("%w: %w", ErrOptions, err)
	}

	s.llm = llm

	return nil
}

// setupElicitManager ensures that the elicitManager, and thus the elicit Tool, have a bubbletea emitter to send
// messages to the UI
func (s *Smoke) setupElicitManager() {
	manager := elicit.NewManager()
	manager.SetOnBegin(func(req elicit.RequestMessage) {
		if s.teaEmitter == nil {
			return
		}

		s.teaEmitter(req)
	})

	s.elicitManager = manager
}

func (s *Smoke) setupSession(ctx context.Context) error {
	slog.Debug("setting up main session", "name", s.mainSessionName)

	mode := modes.DefaultMode()

	toolManager, err := s.NewToolManager(ctx, mode)
	if err != nil {
		return fmt.Errorf("failed to set up tools manager for main smoke session: %w", err)
	}

	systemPrompt, err := s.SystemPrompt(mode)
	if err != nil {
		return fmt.Errorf("failed to get system prompt during setup: %w", err)
	}

	session, err := llms.NewSession(&llms.SessionOpts{
		Name:            s.mainSessionName,
		SystemMessage:   systemPrompt,
		SystemAsMessage: s.llm.RequiresSessionSystem(),
		Tools:           toolManager,
		Mode:            mode,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize main smoke session: %w", err)
	}

	s.sessionMutex.Lock()

	s.sessions[s.mainSessionName] = session

	s.sessionMutex.Unlock()

	return nil
}

func (s *Smoke) setupMCPClients(ctx context.Context) error {
	if s.config == nil || s.config.MCP == nil {
		slog.Debug("no MCP configuration defined")
		return nil
	}

	clients := []*mcp.CommandClient{}

	for _, serverConfig := range s.config.MCP.Servers {
		// Don't initialize clients for servers the user has disabled
		if !serverConfig.Enabled {
			slog.Debug("MCP server is disabled", "name", serverConfig.Name)
			continue
		}

		opts := &mcp.CommandClientOpts{
			MCPServer: serverConfig,
			Directory: s.projectPath,
		}

		client, err := mcp.NewCommandClient(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to set up MCP client %q: %w", opts.Name, err)
		}

		clients = append(clients, client)
	}

	s.mcpClients = clients

	return nil
}

func (s *Smoke) setupCommands() error {
	s.commands = commands.NewManager(s.projectPath)
	s.commands.SetTeaEmitter(s.teaEmitter)

	for commandName, initializer := range cmdhandlers.AllCommands() {
		if err := s.commands.Register(commandName, initializer); err != nil {
			return fmt.Errorf("failed to set up command %q: %w", commandName, err)
		}
	}

	return nil
}
