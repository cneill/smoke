# smoke

[![Go Report Card](https://goreportcard.com/badge/github.com/cneill/smoke)](https://goreportcard.com/report/github.com/cneill/smoke) [![Go package documentation](https://pkg.go.dev/badge/github.com/cneill/smoke)](https://pkg.go.dev/github.com/cneill/smoke)

## About

Smoke is my attempt at an agentic coding assistant for Go. It is tailored to *my* needs. Read more about why I built it
[here](https://techiavellian.com/introducing-smoke/).

The [raink](https://github.com/noperator/raink/) tool described in 
[this blog post](https://bishopfox.com/blog/raink-llms-document-ranking) was the direct inspiration for my `/rank`
command. My implementation is incomplete, but I still find it useful.

If people discover this repo and want to use it, I might write a more helpful README in the future. The code is quite
messy in some places, documentation is sparse to nonexistent, and there are likely many bugs. Caveat emptor.

## Motivation

* Pure hubris

## CLI Usage

`smoke [flags] [project path]`

```
GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version

   LLM Configuration

   --max-tokens int, -t int       The max tokens to return in any given response (default: 8192) [$SMOKE_MAX_TOKENS]
   --model string, -m string      The provider's model to use, or an alias for it [$SMOKE_MODEL]
   --provider string, -p string   One of the following: chatgpt, claude, grok [$SMOKE_PROVIDER]
   --temperature float, -T float  The temperature value to use with the model (default: 1) [$SMOKE_TEMPERATURE]

   Local Configuration

   --debug, -D              Enable debug logging. [$SMOKE_DEBUG]
   --session NAME, -s NAME  The NAME of the session, which will be used to derive the log file and plan file names (default: "session") [$SMOKE_SESSION]

   Providers

   --anthropic-api-key string  The API key for Anthropic. Required when provider is "claude" [$ANTHROPIC_API_KEY]
   --openai-api-key string     The API key for OpenAI. Required when provider is "chatgpt" [$OPENAI_API_KEY]
   --xai-api-key string        The API key for xAI. Required when provider is "grok" [$XAI_API_KEY]
```

### Example invocation

```bash
smoke -D -p chatgpt -m 5.4 ~/projects/example
```

### Example config file

Smoke will create a default config file in `~/.config/smoke/config.json` if one does not already exist. Right now it's
only useful for configuring MCP servers. The default config includes support for the [`gopls` MCP server][gopls_server]
out of the box.


```json
{
    "mcp": {
        "servers": [
            {
                "name": "example",
                "command": "example_tool",
                "args": [
                    "arg1",
                    "arg2"
                ],
                "enabled": false,
                "allowed_tools": [],
                "denied_tools": [
                    "dangerous_*",
                    "evil_tool"
                ],
                "plan_tools": [],
                "env": [
                    {
                        "var": "EXAMPLE_ENV_VAR",
                        "value": "EXAMPLE VALUE"
                    }
                ]
            },
            {
                "name": "gopls",
                "command": "gopls",
                "args": [
                    "mcp"
                ],
                "enabled": true,
                "allowed_tools": [],
                "denied_tools": [],
                "plan_tools": [],
                "env": null
            }
        ]
    }
}
```

[gopls_server]: https://go.dev/gopls/features/mcp
