# Changelog

Versions follow Semantic Versioning. Commit messages follow Conventional Commits.

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
