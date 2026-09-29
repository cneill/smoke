# About this repo

Smoke is a Go terminal-based AI coding assistant using the bubbletea TUI library.

## Structure

* `cmd/smoke` - main CLI command
* `pkg/ask` - allows the LLM to ask questions
* `pkg/commands` - user slash (/) commands
* `pkg/config` - user configuration, mostly for MCP servers for now
* `pkg/fs` - handles file system details
* `pkg/llmctx` - context management like AGENTS.md files, skills, system prompts, and "modes"
* `pkg/llms` - provider-agnostic primitives for working with LLMs (messages, roles, conversations, etc)
* `pkg/mcp` - utilities for working with MCP servers
* `pkg/models` - bubbletea TUI models
* `pkg/plan` - allows the LLM to create plans for what it will do
* `pkg/providers` - API clients for the various LLM providers, conversation handling, details about providers' models
* `pkg/smoke` - main controller for the whole application
* `pkg/tools` - tools provided to the LLM for reading/writing files, linting, etc.
* `pkg/utils` - common utilities for string manipulation/etc

## Conventions

* When writing tests, use `github.com/stretchr/testify` for assertions and use table-driven tests when using different
  inputs to exercise the same functionality instead of breaking into individual tests
* When sanity-checking structs, particularly options structs passed upon initialization of another struct, use methods
  like OK() error {} that DO NOT mutate the struct evaluated. Simply return an error describing any issues. There are
  examples throughout the current codebase.
