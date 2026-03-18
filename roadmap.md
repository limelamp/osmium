# Osmium Roadmap

This roadmap prioritizes stability and maintainability first, then architecture improvements and feature expansion.

## Guiding Principles

- Stabilize critical paths before adding new features.
- Keep behavior backward-compatible for existing users.
- Prefer small, verifiable refactors over large rewrites.
- Add tests around high-risk workflows (migration, install/update, server lifecycle).

---

## Phase 1 (Week 1): Reliability + UX Baseline

### Goals

- Reduce runtime failures in core workflows.
- Eliminate known panic/unsafe paths.
- Improve command/help consistency.

### Tasks

1. **Fix known runtime hazards in TUI flows**
   - Resolve `RemoveFilesModel` index/map misalignment and repeated-delete crash risk.
   - Add bounds checks for cursor and list operations in all screens.
   - Ensure all file operations in TUI paths return surfaced errors.

2. **Harden process lifecycle controls**
   - Centralize lock file creation/removal checks for run/stop paths.
   - Improve stale lock handling (PID exists check before trusting lock file).
   - Standardize graceful stop vs force stop behavior and messaging.

3. **Harden HTTP integrations**
   - Add `StatusCode` checks where missing (`util/setup.go`, `shared/plugin.go`).
   - Add request timeouts via reusable HTTP client.
   - Normalize retry/no-retry behavior and user-facing error messages.

4. **Finish CLI command quality pass**
   - Replace placeholder Cobra descriptions and help text.
   - Validate required args/flags consistently.
   - Keep `add/remove/update/install/track/migrate` UX aligned.

### Definition of Done

- No known panic paths in TUI file removal and core navigation.
- All network requests check status codes and have timeout behavior.
- All command help text is production-ready (no scaffold placeholders).

---

## Phase 2 (Weeks 2–3): Architecture Refactor (Low Risk)

### Goals

- Reduce coupling between TUI and business logic.
- Make code easier to test and extend.
- Break large files into focused units.

### Tasks

1. **Split `internal/shared/plugin.go` into focused services**
   - `modrinth_client.go` (HTTP/API DTOs)
   - `project_installer.go` (install/update/remove)
   - `project_tracker.go` (hash tracking)
   - `migration_service.go` (migration flow)

2. **Extract side effects from TUI update layer**
   - Introduce service interfaces used by TUI models.
   - Move filesystem/network/process operations into service package(s).
   - Keep model update functions focused on state transitions.

3. **Consolidate config I/O workflow**
   - Avoid repeated read/write in tight loops.
   - Add helper patterns for atomic update operations.
   - Guard map initialization to prevent nil map writes.

4. **Normalize logging/printing strategy**
   - Route operational logs through one abstraction.
   - Distinguish user messages, warnings, and debug output.

### Definition of Done

- `plugin.go` no longer acts as a monolithic service file.
- TUI package has minimal direct network/filesystem/process calls.
- Config updates use consistent safe update flow.

---

## Phase 3 (Weeks 3–4): Test Coverage Foundation

### Goals

- Protect critical workflows against regression.
- Enable confident refactors.

### Tasks

1. **Add unit tests for pure logic first**
   - Dependency folder resolution.
   - Loader resolution and query URL building.
   - Migration decision paths and compatibility handling.

2. **Add integration-like tests with mocks/fakes**
   - Mock Modrinth/API calls.
   - Fake filesystem for install/remove/track behavior.
   - Validate config persistence semantics.

3. **Add process/socket behavior tests where feasible**
   - Lock-file behavior and stale lock handling.
   - Command forwarding path (`exec` → socket listener).

4. **Set quality gates**
   - `go test ./...` in CI.
   - Minimum baseline coverage for critical packages.

### Definition of Done

- Critical paths have automated tests and pass in CI.
- Refactoring no longer depends solely on manual verification.

---

## Phase 4 (Month 2): UX + Product Maturity

### Goals

- Make the app easier to use daily.
- Improve visibility and control in long-running server sessions.

### Tasks

1. **TUI usability upgrades**
   - Add scrollable server logs (viewport model).
   - Improve keybind consistency (`backspace` vs `ctrl+h`).
   - Add clear operation states (idle/running/error/success).

2. **Config editor robustness**
   - Replace naive YAML parsing/writing with a safer structured approach.
   - Preserve comments/order where feasible.
   - Validate edits before writing.

3. **Project organization improvements**
   - Optional server directory layout strategy (`mc-servers/<name>`).
   - Better initialization prompts and defaults.

4. **Documentation refresh**
   - Expand README with workflow examples and command matrix.
   - Add troubleshooting section for lock files, migration, and API failures.

### Definition of Done

- TUI interactions are predictable and discoverable.
- Config editing is safer and less destructive.
- Documentation reflects current real behavior.

---

## Backlog (After Stabilization)

- Java runtime detection and setup assistant.
- Optional global command bootstrap flow.
- Advanced plugin/mod search UX and richer query support.
- Telemetry-free local diagnostics bundle (`osmium doctor`).

---

## Suggested Execution Order (Top 10)

1. Fix RemoveFiles panic/indexing and cursor safety.
2. Add HTTP status + timeout handling everywhere.
3. Improve lock file and stale PID behavior.
4. Replace all placeholder Cobra command text.
5. Split `plugin.go` into service-focused files.
6. Move side effects out of TUI update methods.
7. Add unit tests for dependency/query/migration logic.
8. Add API+filesystem mock-based tests.
9. Add scrollable server log viewport.
10. Replace naive YAML editing with structured parsing.

---

## Success Metrics

- Fewer user-reported runtime crashes in core workflows.
- Faster onboarding for contributors (smaller, clearer files).
- Stable command behavior across CLI and TUI paths.
- Measurable increase in confidence via automated tests.
