# smoke

## About

Smoke is my attempt at an agentic coding assistant. It is tailored to *my* needs, and is best viewed as a hobby research
project. To avoid creating a poor simulacrum of other existing coding assistants, I have not allowed myself to install
or use any others up until this point. While I have taken inspiration from various sources such as Thorsten Ball's [How
to Build an Agent](https://ampcode.com/how-to-build-an-agent) blog post, I have used only smoke to directly help me
build smoke. This likely means that I am missing features considered table stakes in the broader coding assistant space,
but this is by design.

The [raink](https://github.com/noperator/raink/) tool described in 
[this blog post](https://bishopfox.com/blog/raink-llms-document-ranking) was the direct inspiration for my `/rank`
command. My implementation is incomplete, but I still find it useful.

If people discover this repo and express interest in using smoke, I might write a more helpful README in the future. The
code is quite messy in some places, documentation is sparse to nonexistent, and there are likely many bugs. Caveat
emptor.

## Motivations

* I wanted to better understand LLM tool use
* This seemed like a fun project to pursue while I was between jobs
* I doubted that existing tools would have established insurmountable moats after only a few months of existence
* If I were to take claims about the *massive* future importance of these tools at face value, it seemed absurd to let
  others decide how I could use them
* Conversations between Dax and Adam on [@TerminalDotShop's YouTube channel](https://www.youtube.com/@TerminalDotShop)
  about their experience building [opencode](https://github.com/sst/opencode) suggested to me that creators of these
  tools are often just watching each other and copying popular features. I felt that this would lead them all to take
  similar approaches when that might not be warranted
    * To be clear, **I am not criticizing opencode**, and I look forward to taking it for a test drive myself. I simply
      agree with much of *their* skepticism about the breathless online discourse around these tools. They're not magic,
      and it is refreshing to hear people with skin in the game say as much
* Pure hubris

## CLI Usage

```
NAME:
   smoke - Smoke 'em if you got 'em.

USAGE:
   smoke [global options]

VERSION:
   v0.0.2

DESCRIPTION:
   An agentic coding assistant primarily focused on the Go programming language. It only works on one directory at a time, and that directory must contain a .git subdirectory.

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version

   LLM Configuration

   --max-tokens int, -t int       The max tokens to return in any given response (default: 8192) [$SMOKE_MAX_TOKENS]
   --model string, -m string      The provider's model to use, or an alias for it [$SMOKE_MODEL]
   --provider string, -p string   One of the following: chatgpt, claude, grok [$SMOKE_PROVIDER]
   --temperature float, -T float  The temperature value to use with the model (default: 1) [$SMOKE_TEMPERATURE]

   Local Configuration

   --debug, -D                    Enable debug logging. (default: false) [$SMOKE_DEBUG]
   --dir DIRECTORY, -d DIRECTORY  The DIRECTORY where your project lives. [$SMOKE_DIRECTORY]
   --session NAME, -s NAME        The NAME of the session, which will be used to derive the log file and plan file names (default: "session") [$SMOKE_SESSION]

   Providers

   --anthropic-api-key string  The API key for Anthropic. Required when provider is "claude" [$ANTHROPIC_API_KEY]
   --openai-api-key string     The API key for OpenAI. Required when provider is "chatgpt" [$OPENAI_API_KEY]
   --xai-api-key string        The API key for xAI. Required when provider is "grok" [$XAI_API_KEY]
```

### Example invocation

```bash
smoke -D -t 8192 -T 1 -d . -p grok -m code
```

### Example config file

Smoke will create a config file in `~/.config/smoke/config.json` if one does not already exist. The API key fields do
not actually do anything at this point. It is primarily useful for defining MCP servers. The default config file
contains a fake MCP server that you'll want to delete.

```json
{
  "providers": {
    "anthropic_key": "",
    "openai_key": "",
    "xai_key": ""
  },
  "mcp": {
    "servers": [
      {
        "name": "gopls",
        "command": "gopls",
        "args": ["mcp"],
        "enabled": true,
        "allowed_tools": ["go_*"],
        "denied_tools": [],
        "plan_tools": ["go_*"],
        "env": null
      }
    ]
  }
}
```
