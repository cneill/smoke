package smoke

import (
	"fmt"

	"github.com/cneill/smoke/pkg/commands"
	"github.com/cneill/smoke/pkg/commands/handlers/edit"
	"github.com/cneill/smoke/pkg/commands/handlers/exit"
	"github.com/cneill/smoke/pkg/commands/handlers/export"
	"github.com/cneill/smoke/pkg/commands/handlers/help"
	"github.com/cneill/smoke/pkg/commands/handlers/info"
	"github.com/cneill/smoke/pkg/commands/handlers/load"
	plancmd "github.com/cneill/smoke/pkg/commands/handlers/plan"
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
	"github.com/cneill/smoke/pkg/tools/handlers/ddg"
	gitdiff "github.com/cneill/smoke/pkg/tools/handlers/git/diff"
	"github.com/cneill/smoke/pkg/tools/handlers/gofumpt"
	"github.com/cneill/smoke/pkg/tools/handlers/goimports"
	"github.com/cneill/smoke/pkg/tools/handlers/golint"
	"github.com/cneill/smoke/pkg/tools/handlers/gotest"
	"github.com/cneill/smoke/pkg/tools/handlers/grep"
	"github.com/cneill/smoke/pkg/tools/handlers/listfiles"
	"github.com/cneill/smoke/pkg/tools/handlers/mkdir"
	planadd "github.com/cneill/smoke/pkg/tools/handlers/plan/add"
	plancompletion "github.com/cneill/smoke/pkg/tools/handlers/plan/completion"
	planread "github.com/cneill/smoke/pkg/tools/handlers/plan/read"
	planupdate "github.com/cneill/smoke/pkg/tools/handlers/plan/update"
	"github.com/cneill/smoke/pkg/tools/handlers/readfile"
	"github.com/cneill/smoke/pkg/tools/handlers/writefile"
)

// TODO: move command / tool handler imports to their "handlers" packages?

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
		Mode:            llms.ModeNormal,
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

	commands := map[string]commands.Initializer{
		edit.Name:      edit.New,
		exit.Name:      exit.New,
		export.Name:    export.New,
		help.Name:      help.New(s.commands),
		info.Name:      info.New,
		load.Name:      load.New,
		plancmd.Name:   plancmd.New,
		review.Name:    review.New,
		run.Name:       run.New,
		save.Name:      save.New,
		session.Name:   session.New,
		summarize.Name: summarize.New,
	}

	for commandName, initializer := range commands {
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
		case llms.ModeNormal:
			initList = s.normalModeTools()
		case llms.ModePlanning:
			initList = s.planningModeTools()
		case llms.ModeReview:
			initList = s.reviewModeTools()
		}
	} else {
		initList = s.normalModeTools()
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

	return toolManager, nil
}

func (s *Smoke) normalModeTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
		gitdiff.New,
		gofumpt.New,
		goimports.New,
		golint.New,
		gotest.New,
		grep.New,
		listfiles.New,
		mkdir.New,
		planadd.New,
		plancompletion.New,
		planread.New,
		planupdate.New,
		readfile.New,
		writefile.New,
	}
}

func (s *Smoke) planningModeTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
		gitdiff.New,
		golint.New,
		gotest.New,
		grep.New,
		listfiles.New,
		planadd.New,
		plancompletion.New,
		planread.New,
		planupdate.New,
		readfile.New,
	}
}

func (s *Smoke) reviewModeTools() []tools.Initializer {
	return []tools.Initializer{
		ddg.New,
		gitdiff.New,
		golint.New,
		gotest.New,
		grep.New,
		listfiles.New,
		planadd.New,
		plancompletion.New, // TODO: ?
		planread.New,
		planupdate.New,
		readfile.New,
	}
}

func (s *Smoke) summarizeTools() []tools.Initializer {
	return []tools.Initializer{
		planread.New,
	}
}
