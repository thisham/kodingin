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

REPL: `/clear`, `/yolo`, `/help`, `/exit`; trailing `\` for multi-line; Ctrl-C cancels a turn.

## Layout (the agent's own code is DDD too)
    cmd/kodingin/                 composition root
    internal/domain/conversation  Conversation aggregate, Message, ToolCall
    internal/domain/tool          Tool interface + Registry
    internal/domain/llm           llm.Client port
    internal/application          Agent use case (model ↔ tool loop), system prompt
    internal/infrastructure/llm   tokenizer.id adapter
    internal/infrastructure/tools read/write/edit/list/grep/shell/go_check/go_test (sandboxed to -C)
    internal/interfaces/cli       REPL, colours, diff preview, approval prompts
