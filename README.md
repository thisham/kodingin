# kodingin

CLI coding agent for Go/DDD projects, backed by `https://tokenizer.id/v1` (OpenAI-compatible `/chat/completions` + tool calling).

## Setup
    go build -o kodingin ./cmd/kodingin
    mkdir -p ~/.config/kodingin && echo 'KODINGIN_API_KEY=sk-...' > ~/.config/kodingin/env
    # optional: KODINGIN_MODEL=..., KODINGIN_BASE_URL=https://tokenizer.id/v1

## Use
    ./kodingin -C /path/to/project            # interactive REPL
    ./kodingin -C . -p "add a User aggregate"  # one shot
    ./kodingin -yes ...                        # auto-approve writes/shell

REPL: `/help`, `/status`, `/version`, `/model [id]`, `/setup`, `/api-key`, `/api-key-set`, `/clear`, `/yolo`, `/exit`. In interactive terminals, Enter submits and Alt+Enter or Ctrl-J inserts a newline; Ctrl-C cancels a turn. `/setup` modifies the saved endpoint and model in `~/.config/kodingin/env`; restart to apply. `/clear` resets conversation and session usage.

In an interactive terminal, the REPL opens a full-screen transcript with the input pinned at the bottom. Use Page Up and Page Down to scroll, or Ctrl-D on an empty prompt to exit. Tool approvals and hidden API-key entry appear on the bottom row. Piped input, redirected output, and `TERM=dumb` use the line-based REPL.

The welcome screen shows the version, model, workspace, and approval mode. Each turn reports elapsed time, token usage, and session totals. Tool output uses compact previews. Approval requires `y` (once), `n` (deny, the default), or `a` (auto-approve for the session).

Use `./kodingin --version` to check the version without an API key. Redirected output has no ANSI styling. Set `NO_COLOR=1` to disable color in a terminal.

Commit messages follow **Conventional Commits** and releases follow **Semantic Versioning**. See [CONTRIBUTING.md](CONTRIBUTING.md) for commit types and release steps, and [CHANGELOG.md](CHANGELOG.md) for version history.

## Layout (the agent's own code is DDD too)
    cmd/kodingin/                 composition root
    internal/domain/conversation  Conversation aggregate, Message, ToolCall
    internal/domain/tool          Tool interface + Registry
    internal/domain/llm           llm.Client port
    internal/application          Agent use case (model ↔ tool loop), system prompt
    internal/infrastructure/llm   tokenizer.id adapter
    internal/infrastructure/tools read/write/edit/list/grep/shell/go_check/go_test (sandboxed to -C)
    internal/interfaces/cli       REPL, colours, diff preview, approval prompts
