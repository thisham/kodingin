# Changelog

Versions follow Semantic Versioning. Commit messages follow Conventional Commits.

## 0.5.0 (2026-11-20)

- Style the pinned CLI composer like a chat input: rules above and below, one line that grows with the prompt, aligned continuation lines, and working status above the input.

## 0.5.0-rc.5 (2026-11-20)

- Style the pinned CLI composer like a chat input: rules above and below, one line that grows with the prompt, aligned continuation lines, and working status above the input.

## 0.5.0-rc.4 (2026-11-20)

- Keep the pinned CLI composer in a dedicated three-line area with status and footer separated from input.
- Use Esc to cancel a running turn; show the cancel hint and sovcore copyright in the footer.

## 0.5.0-rc.3 (2026-11-20)

- Navigate previously submitted prompts with ↑/↓ in the pinned CLI; restore the unsent draft after browsing history.
- Keep approval/input prompts active when a task completes and process queued work afterward.
- Show live token usage and elapsed time while agent tasks run.

## 0.5.0-rc.2 (2026-11-20)

- Bound retained conversation history to complete recent turns.
- Bound in-memory session usage records to the latest 1,000 model calls.
- Improve pinned CLI prompt wrapping and transcript navigation.

## 0.5.0-rc.1 (2026-11-20)

- Add an autonomous harness mode that emits JSON traces for CI and agent evaluation.
- Add configurable harness timeouts and bounded tool output in reports.

## 0.4.1 (2026-11-18)

- Release discarded conversation memory when clearing, compacting, restoring, or trimming interrupted tool calls.
- Bound terminal scrollback to 256 KiB while preserving UTF-8, terminal escape boundaries, and scroll position.
- Capture at most 64 KiB of command output, drain the remainder, and report truncation.
- Close HTTP response bodies on retry cancellation and skip the wait after the final attempt.

## 0.4.0 (2026-11-18)

- Scroll the output with the mouse wheel, Shift+↑/↓ (or ↑/↓ while the input is empty), Page Up/Down and Ctrl+Home. Esc or Ctrl+End jumps back to the latest output.
- While scrolled up, the divider shows how many lines are below; new output no longer pulls the view down.
- Mouse capture takes over click-and-drag selection (hold Option in iTerm2 or Fn in Terminal.app to select). Set `KODINGIN_NO_MOUSE=1` to turn it off.

## 0.3.0 (2026-11-18)

- Conversations are saved after each turn to `~/.config/kodingin/sessions/` (mode 600).
- `/resume` lists recent sessions; `/resume <n>` continues one and shows where you left off. `kodingin -continue` resumes the newest at startup.
- `/compact [focus]` replaces the history with model-written handover notes to free context. The history is kept if the summary fails or is not smaller.
- `/clear` starts a new session; the previous one stays saved.
- Resumed sessions with an interrupted tool call are trimmed so the model accepts them.

## 0.2.0 (2026-11-18)

- `/usage` shows tokens per model for this session (input, cached, output) and today / 7 days / 30 days from `~/.config/kodingin/usage.jsonl`.
- `/usage-credit` shows account spend, budget limits and the token package from the tokenizer.id portal. It needs a portal browser session: `/usage-credit login` saves the Cookie only after the portal accepts it, and `/usage-credit logout` removes it.
- `/history [n]` lists recent prompts across sessions from `~/.config/kodingin/history.jsonl`. Slash commands are never recorded.
- Cached prompt tokens are read from API responses.

## 0.1.1 (2026-11-18)

- Pinned full-screen terminal UI with scrollable transcript and a fixed input box.
- `/setup` to configure the saved endpoint and model.
- Enter submits the prompt. Alt+Enter or Ctrl-J inserts a newline. Previously Enter inserted a newline and only Ctrl-J submitted.

## 0.1.0

Initial versioned baseline. This entry does not imply a published release or tag.

- Go CLI coding agent with model switching, API-key management, tool previews, and approval prompts.
- Welcome screen with model, workspace, approval mode, and version.
- Grouped help, `/status`, `/version`, and `--version`.
- Per-turn elapsed time and token usage alongside session totals.
- Plain output when redirected, when `NO_COLOR` is set, or when `TERM=dumb`.
- Unicode-safe tool previews and explicit approval choices. Empty input denies a tool action.
