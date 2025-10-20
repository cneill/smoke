package smoke

import (
	"fmt"

	"github.com/cneill/smoke/pkg/commands"
	cmdhandlers "github.com/cneill/smoke/pkg/commands/handlers"
	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/llms"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/cneill/smoke/pkg/providers/chatgpt"
	"github.com/cneill/smoke/pkg/providers/claude"
	"github.com/cneill/smoke/pkg/providers/grok"
	"github.com/cneill/smoke/pkg/tools"
	toolhandlers "github.com/cneill/smoke/pkg/tools/handlers"
)

func (s *Smoke) setup() error {
	if err := s.OK(); err != nil {
		return fmt.Errorf("%w: %w", ErrOptions, err)
	}

	if err := s.setupPlanManager(); err != nil {
		return fmt.Errorf("failed to set up plan manager for smoke: %w", err)
	}

	if err := s.setupLLM(); err != nil {
		return fmt.Errorf("failed to set up LLM: %w", err)
	}

	if err := s.setupSession(); err != nil {
		return fmt.Errorf("failed to set up main smoke session: %w", err)
	}

	if err := s.setupMCP(); err != nil {
		return fmt.Errorf("failed to set up MCP servers: %w", err)
	}

	s.setupCommands()

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

	s.llm = llm

	return nil
}

func (s *Smoke) setupSession() error {
	toolManager, err := s.setupToolsManager()
	if err != nil {
		return fmt.Errorf("failed to set up tools manager for main smoke session: %w", err)
	}

	session, err := llms.NewSession(&llms.SessionOpts{
		Name:            s.mainSessionName,
		SystemMessage:   s.mainSystemPrompt,
		Tools:           toolManager,
		SystemAsMessage: s.llm.RequiresSessionSystem(),
		Mode:            llms.ModeWork,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize main smoke session: %w", err)
	}

	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	s.sessions[s.mainSessionName] = session

	return nil
}

func (s *Smoke) setupMCP() error {
	// Once we've set up the session / etc, add MCP tools as well, if any
	mcpTools, err := s.getMCPTools()
	if err != nil {
		return fmt.Errorf("failed to list MCP tools: %w", err)
	}

	session := s.getMainSession()

	session.Tools.AddTools(mcpTools...)

	return nil
}

func (s *Smoke) setupCommands() {
	s.commands = commands.NewManager(s.projectPath)

	for commandName, initializer := range cmdhandlers.AllCommands() {
		s.commands.Register(commandName, initializer)
	}
}

func (s *Smoke) setupToolsManager() (*tools.Manager, error) {
	var (
		initList []tools.Initializer
		session  = s.getMainSession()
	)

	if session != nil {
		switch session.GetMode() {
		case llms.ModeWork:
			// TODO: rename "normal" to "work"
			initList = toolhandlers.NormalTools()
		case llms.ModePlanning:
			initList = toolhandlers.PlanningTools()
		case llms.ModeReview:
			initList = toolhandlers.ReviewTools()
		case llms.ModeSummarize:
			initList = toolhandlers.SummarizeTools()
		}
	} else {
		initList = toolhandlers.NormalTools()
	}

	toolOpts := &tools.ManagerOpts{
		ProjectPath:      s.projectPath,
		SessionName:      s.mainSessionName,
		ToolInitializers: initList,
		PlanManager:      s.planManager,
	}

	toolManager, err := tools.NewManager(toolOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools manager for main smoke session: %w", err)
	}

	if s.teaEmitter != nil {
		toolManager.SetTeaEmitter(s.teaEmitter)
	}

	return toolManager, nil
}
