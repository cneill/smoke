package smoke

import (
	"fmt"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/edit"
	"github.com/cneill/smoke/pkg/commands/handlers/exit"
	"github.com/cneill/smoke/pkg/commands/handlers/export"
	"github.com/cneill/smoke/pkg/commands/handlers/info"
	"github.com/cneill/smoke/pkg/commands/handlers/load"
	planhandler "github.com/cneill/smoke/pkg/commands/handlers/plan"
	"github.com/cneill/smoke/pkg/commands/handlers/review"
	"github.com/cneill/smoke/pkg/commands/handlers/run"
	"github.com/cneill/smoke/pkg/commands/handlers/save"
	"github.com/cneill/smoke/pkg/commands/handlers/session"
	"github.com/cneill/smoke/pkg/commands/handlers/summarize"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/providers/chatgpt"
	"github.com/cneill/smoke/pkg/providers/claude"
	"github.com/cneill/smoke/pkg/providers/grok"
	"github.com/cneill/smoke/pkg/tools"
)

func (s *Smoke) setup() error {
	if err := s.OK(); err != nil {
		return fmt.Errorf("%w: %w", ErrOptions, err)
	}

	if err := s.setupPlanManager(); err != nil {
		return fmt.Errorf("failed to set up plan manager for smoke: %w", err)
	}

	if err := s.setupSession(); err != nil {
		return fmt.Errorf("failed to set up main smoke session: %w", err)
	}

	s.setupCommands()

	if err := s.setupLLM(); err != nil {
		return fmt.Errorf("failed to set up LLM: %w", err)
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

func (s *Smoke) setupSession() error {
	toolOpts := &tools.ManagerOpts{
		ProjectPath:      s.projectPath,
		SessionName:      s.mainSessionName,
		ToolInitializers: tools.AllTools(),
		PlanManager:      s.planManager,
	}

	toolManager, err := tools.NewManager(toolOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize tools manager for main smoke session: %w", err)
	}

	session, err := llms.NewSession(&llms.SessionOpts{
		Name:          s.mainSessionName,
		SystemMessage: s.mainSystemPrompt,
		Tools:         toolManager,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize main smoke session: %w", err)
	}

	// Once we've set up the session / etc, add MCP tools as well, if any
	mcpTools, err := s.getMCPTools()
	if err != nil {
		return fmt.Errorf("failed to list MCP tools: %w", err)
	}

	session.Tools.AddTools(mcpTools...)

	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	s.sessions[s.mainSessionName] = session

	return nil
}

func (s *Smoke) setupCommands() {
	s.commands = commands.NewManager(s.projectPath)

	commands := map[string]commands.Initializer{
		edit.Name:        edit.New,
		exit.Name:        exit.New,
		export.Name:      export.New,
		info.Name:        info.New,
		load.Name:        load.New,
		planhandler.Name: planhandler.New,
		review.Name:      review.New,
		run.Name:         run.New,
		save.Name:        save.New,
		session.Name:     session.New,
		summarize.Name:   summarize.New,
	}

	for commandName, initializer := range commands {
		s.commands.Register(commandName, initializer)
	}
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
	default:
		err = fmt.Errorf("unknown provider: %s", s.llmConfig.Provider)
	}

	if err != nil {
		return fmt.Errorf("%w: %w", ErrOptions, err)
	}

	// Update the session with a system message if needed by this provider
	// TODO: HANDLE THIS IN A TIDIER WAY - STICK IT IN THE LLM PROVIDER?
	if llm.RequiresSessionSystem() {
		session := s.getMainSession()

		session.SystemAsMessage = true
		if err := session.SetSystemMessage(session.SystemMessage); err != nil {
			return fmt.Errorf("failed to update main session system prompt: %w", err)
		}
	}

	s.llm = llm

	return nil
}
