# Contributing

Use idiomatic Go and keep dependencies pointing inward: delivery and infrastructure depend on application and domain. Keep `cmd/kodingin` focused on wiring.

Run `gofmt -w cmd internal`, `go vet ./...`, `go build ./...`, and `go test ./...` before submitting changes. Use table-driven tests for domain and application behavior.

## Commit messages

Use [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```text
type(scope): short description
```

Scope is optional. Use lowercase types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, or `revert`.

Examples:

```text
feat(cli): add session status command
fix(cli): preserve unicode in tool previews
docs: explain release versioning
feat(cli)!: change approval command syntax
```

Mark breaking changes with `!` after the type/scope or a `BREAKING CHANGE:` footer explaining the migration. Only create commits when requested.

## Releases

Use [Semantic Versioning 2.0.0](https://semver.org/). The release number lives in `internal/version/VERSION` and is embedded in the binary. `--version`, `/version`, and the welcome screen show that same value. The initial version is `0.1.0`.

For stable releases, increment MAJOR for incompatible changes, MINOR for backward-compatible features, and PATCH for backward-compatible fixes. During `0.x` development, increment MINOR for breaking changes or features and PATCH for fixes; record breaking changes explicitly. Documentation and internal maintenance alone do not require a release.

When a release is requested:

1. Choose the next version based on the changes since the previous release.
2. Update `internal/version/VERSION` and add a dated entry to `CHANGELOG.md`, including migration notes for breaking changes.
3. Run the checks above and verify `go run ./cmd/kodingin --version`.
4. Commit as `chore(release): X.Y.Z` and tag `vX.Y.Z` when tagging is requested.

Do not bump versions, tag, or publish automatically after ordinary changes.
