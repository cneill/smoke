# About this repo

Smoke is a Go terminal-based AI coding assistant using the bubbletea TUI library.

## Structure

* `cmd/smoke` - main CLI command
* `internal/log` - global slog setup
* `internal/uimsg` - shared bubbletea helpers (`MsgToCmd`, `TeaEmitter`, UI `Error` type)
* `internal/version` - version string from ldflags or Go build info
* `pkg/ask` - allows the LLM to ask questions
* `pkg/commands` - user slash (/) command manager
  * `handlers/*` - one package per command (edit, export, mode, plan, rank, summarize, etc)
* `pkg/config` - user config at `$XDG_CONFIG_HOME/smoke/config.json` (MCP servers for now), config/plans dir paths
* `pkg/fs` - project path containment checks, `.smokeignore` exclusions, `@` path completion
* `pkg/llmctx` - context management
  * `agentsmd` - AGENTS.md discovery
  * `modes` - operating modes (work, planning, review, plus internal summarize/ranking)
  * `prompts` - system prompt building and rendering
  * `skills` - skill discovery and catalog
* `pkg/llms` - provider-agnostic primitives: `LLM`/`Conversation` interfaces, `Session` (history, usage, mode), messages
  and content blocks, streaming events, tool calls, model info
* `pkg/mcp` - utilities for working with MCP servers
* `pkg/models` - bubbletea TUI models
  * `ui` - root model
  * `history` - conversation history rendering
  * `input` - prompt input with vim keybindings and completions
  * `statusline`, `planpicker`, `banner` - smaller UI components
* `pkg/plan` - LLM plans (tasks, context, completions), persisted per project so they can be resumed
* `pkg/providers` - provider/model registry (defaults, aliases, context windows, effort options)
  * `base` - shared conversation logic
  * `chatgpt`, `claude`, `grok`, `ollama` - provider API clients and conversation handling
* `pkg/smoke` - main controller: sessions, conversations, modes, per-mode tool setup, MCP clients, plans,
  summarize/rank flows
* `pkg/tools` - LLM tool framework (`Manager`, args/params schemas)
  * `handlers/*` - one package per tool (file ops, grep, Go tooling, git diff, Playwright, plan, ask, skills, etc)
* `pkg/utils` - common utilities for string manipulation/etc

## Conventions

* When writing tests, use `github.com/stretchr/testify` for assertions and use table-driven tests when using different
  inputs to exercise the same functionality instead of breaking into individual tests
* When sanity-checking structs, particularly options structs passed upon initialization of another struct, use methods
  like OK() error {} that DO NOT mutate the struct evaluated. Simply return an error describing any issues. There are
  examples throughout the current codebase.
