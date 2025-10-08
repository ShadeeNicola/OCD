# Repository Guidelines

## Project Structure & Module Organization
- `app/` hosts the Go module; `cmd/ocd-gui/main.go` is the desktop entry point and `internal/` contains packages for config, executors, scheduling, http handlers, security, and embedded UI helpers.
- Static assets live under `app/web/` (HTML, JS, CSS) and `app/icons/`; Go embeds them during builds via `internal/ui`.
- `build/` provides automation scripts (see `build-all-executables.sh`), `deploy-scripts/` bundles shell helpers that ship with releases, `docs/` captures architecture notes, and `dist/` is the generated artifact drop.

## Build, Test, and Development Commands
- `DIST_DIR=dist ./build/build-all-executables.sh` — cross-compiles platform binaries using Go 1.21 as in CI.
- `(cd app && go build -o ../dist/OCD.exe ./cmd/ocd-gui)` — fast local Windows build; swap `GOOS/GOARCH` to target other platforms.
- `(cd app && go run ./cmd/ocd-gui)` — launch the GUI against local assets for iterative development.
- `(cd app && go test ./...)` — run Go unit tests (add `_test.go` files beside packages). Keep stdout clean.

## Coding Style & Naming Conventions
- Format Go code with `gofmt` (tabs for indentation); organise imports with `goimports`. Package names stay short, lowercase, and align with folder names in `internal/`.
- Prefer small, composable functions; keep exported identifiers Go-style `CamelCase`. Match Go error patterns (`return err`).
- Frontend JS uses ES modules in `app/web/js/`; follow camelCase for variables and PascalCase for constructor-like functions.
- Bind new assets through Go embed directives; maintain mirrored path names so updates remain predictable.

## Testing Guidelines
- Use Go’s `testing` package; name files `<feature>_test.go` and test functions `TestFeatureBehavior`.
- Mock external shell execution via interfaces in `internal/executor`; isolate network calls with fakes under `internal/httpclient`.
- Record regression scenarios for deployment scripts inside `deploy-scripts/scripts` with shellcheck-compatible helpers when feasible.

## Commit & Pull Request Guidelines
- Write imperative commit subjects (e.g., `Fix version detection`); include optional scope prefixes such as `ci:` or `chore:` when helpful.
- Group related changes per commit; keep diffs focused and mention tooling commands in the body if they informed the change.
- PRs should describe motivation, testing evidence (`go test ./...`, manual GUI checks), and reference issues or release goals. Attach screenshots when UI-visible behavior changes.
- Ensure CI passes on GitHub Actions’ release workflow; regenerate `dist/` artifacts only when intentional.
