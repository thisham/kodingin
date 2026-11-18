# Changelog

Versions follow Semantic Versioning. Commit messages follow Conventional Commits.

## 0.1.0

Initial versioned baseline. This entry does not imply a published release or tag.

- Go CLI coding agent with model switching, API-key management, tool previews, and approval prompts.
- Welcome screen with model, workspace, approval mode, and version.
- Grouped help, `/status`, `/version`, and `--version`.
- Per-turn elapsed time and token usage alongside session totals.
- Plain output when redirected, when `NO_COLOR` is set, or when `TERM=dumb`.
- Unicode-safe tool previews and explicit approval choices. Empty input denies a tool action.
