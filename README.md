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

REPL: `/help`, `/status`, `/resume [n]`, `/compact [focus]`, `/history [n]`, `/usage`, `/usage-credit [login|logout]`, `/version`, `/model [id]`, `/setup`, `/api-key`, `/api-key-set`, `/clear`, `/yolo`, `/exit`. In interactive terminals, Enter submits and Alt+Enter or Ctrl-J inserts a newline; Ctrl-C cancels a turn. `/setup` modifies the saved endpoint and model in `~/.config/kodingin/env`; restart to apply. `/clear` resets conversation and session usage.

In an interactive terminal, the REPL opens a full-screen transcript with the input pinned at the bottom. Scroll the output with the mouse wheel, Page Up/Page Down, Shift+↑/↓ (or ↑/↓ while the input is empty), and Ctrl+Home for the top; Esc or Ctrl+End jumps back to the latest output. While scrolled up, the divider shows how many lines are below. Mouse capture takes over click-and-drag selection (hold Option in iTerm2 or Fn in Terminal.app to select), and `KODINGIN_NO_MOUSE=1` turns it off. Ctrl-D on an empty prompt exits. Tool approvals and hidden API-key entry appear on the bottom row. Piped input, redirected output, and `TERM=dumb` use the line-based REPL.

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

## Usage and credit

- `/usage` shows tokens for this session per model (input, cached, output), plus today / 7 days / 30 days from `~/.config/kodingin/usage.jsonl`. Every model call is appended there (mode 600). These are kodingin's own counts on this machine.
- `/usage-credit` shows the account view from the tokenizer.id portal: this month's spend, the last 30 days, budget limits with what is left, and the token package. The portal does not accept the API key, so it needs your browser session: run `/usage-credit login`, then in a browser logged in to tokenizer.id open DevTools → Network → any `/portal/api` request → Request Headers and paste the `Cookie` value at the hidden prompt. It is saved as `KODINGIN_PORTAL_COOKIE` in `~/.config/kodingin/env` only after the portal accepts it. When the session expires, log in again. `/usage-credit logout` removes it.

## Prompt history

`/history` lists your 20 most recent prompts across sessions (`/history 50` for more), newest last with the latest numbered 1, showing when and, if different from the current one, in which workspace. Prompts sent to the model are appended to `~/.config/kodingin/history.jsonl` (mode 600). Slash commands, including `/api-key-set` and `/usage-credit`, are never recorded. Prompts are stored as typed, so avoid pasting secrets into them.

## Sessions

Every conversation is saved after each turn to `~/.config/kodingin/sessions/<id>.json` (mode 600).

- `/resume` lists recent sessions (title from the first prompt, when, turns, and the workspace if it differs). `/resume <n>` continues one and shows the last prompt and answer. `kodingin -continue` resumes the newest session at startup.
- `/clear` starts a new conversation; the previous one stays saved.
- `/compact [focus]` asks the model for handover notes (goals, decisions, files changed, current state, next steps) and replaces the history with them, freeing context. Add a focus to stress something, for example `/compact the failing tests`. If the summary is not smaller than the history, the history is kept.

A resumed session keeps working in the current `-C` folder and with the current model; kodingin notes when either differs from the original.

