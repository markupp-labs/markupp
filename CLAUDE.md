## Code style

- Functions: 4-20 lines. Split if longer.
- Files: under 500 lines. Split by responsibility.
- One thing per function, one responsibility per module (SRP).
- Names: specific and unique. Avoid `data`, `handler`, `Manager`.
  Prefer names that return <5 grep hits in the codebase.
- Types: explicit. No `any`, no `Dict`, no untyped functions.
- No code duplication. Extract shared logic into a function/module.
- Early returns over nested ifs. Max 2 levels of indentation.
- Exception messages must include the offending value and expected shape.
- No comments. Clear names and short functions replace them.

## Tests

- Tests run with a single command: `<project-specific>`.
- Every new function gets a test. Bug fixes get a regression test.
- Mock external I/O (API, DB, filesystem) with named fake classes,
  not inline stubs.
- Tests must be F.I.R.S.T: fast, independent, repeatable,
  self-validating, timely.
- TDD per commit: the first commit of a task brings only the failing
  tests; later commits make them pass.

## Dependencies

- Inject dependencies through constructor/parameter, not global/import.
- Wrap third-party libs behind a thin interface owned by this project.

## Structure

- Follow the framework's convention (Rails, Django, Next.js, etc.).
- Prefer small focused modules over god files.
- Predictable paths: controller/model/view, src/lib/test, etc.

## Formatting

- Use the language default formatter (`cargo fmt`, `gofmt`, `prettier`,
  `black`, `rubocop -A`). Don't discuss style beyond that.
- Before every commit, run `gofmt -l`, `go vet`, and `golangci-lint run`;
  fix everything before committing.

## Logging

- Structured JSON when logging for debugging / observability.
- Plain text only for user-facing CLI output.

## Git Conventions

**Atomic, frequent commits.** One logical change per commit. If the message needs "and", split it. Commit as soon as a unit of work is coherent and tests pass; don't batch unrelated changes. Refactor, feature, and fix go in separate commits even when touching the same file.

**Commit message format.** Conventional Commits, written in Portuguese, no scope in parentheses: `feat:`, `fix:`, `docs:`, `test:`, `chore:`. Use `refact:` for refactors, never `refactor:`. No emojis, no em dashes in messages. Example: `feat: adiciona validação de upload`.

**Branches and PRs.** Branch prefixes: `feat/`, `fix/`, `refactor/` (never `feature/`, even if old merges show it). Feature work targets `dev` (`--base dev`); only release or milestone consolidation targets `main`. Promote `dev` to `main` with a merge commit, never squash, then back-merge `main` into `dev`.

**Templates.** Every issue and pull request must follow the templates in `.github` (`.github/ISSUE_TEMPLATE/` and `.github/PULL_REQUEST_TEMPLATE.md`).

**AI agents never take credit, in any artifact.** An agent is a tool the human uses; the human is the author of the work. Concretely, when authoring or editing on behalf of the user:

- Never add `Co-Authored-By` trailers to commits.
- Never list yourself (or any model/tool name) as author, co-author, contributor, decider, reviewer, or signer in frontmatter, YAML headers, author lists, READMEs, ADRs, docstrings, code comments, changelogs, release notes, PR descriptions, or any other versioned artifact.
- Never insert "🤖 Generated with..." or similar attribution footers in commits, PR bodies, or generated files.
- If a template field expects an author/decider/owner, leave it for the human to fill or omit the field — do not put yourself there as a placeholder.
- If asked to remove existing credit attributions, remove them in full rather than substituting another agent name.

The rule applies whether the work is shipped, scaffolded, drafted, or merely staged.

## Writing style

- No emojis anywhere, including repo templates that ship with them.
- No em dashes (`—`); use commas, colons, parentheses, or rephrase.
- Backticks sparingly in prose: mark a term once, not every repetition.

## Decisions

- Decisions belong to the user. List the options and wait for the choice;
  don't fill gaps with reasonable defaults, even small ones.

## Agent skills

### Issue tracker

Issues live as GitHub issues on `markupp-labs/markupp`, driven by the `gh` CLI.
See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings; `bug` and `enhancement`
carry the category roles. See `docs/agents/triage-labels.md`.

### Domain docs

Multi-context: a root `CONTEXT-MAP.md` points at one `CONTEXT.md` per module
(`markupp/`, `obsidian-plugin/`). See `docs/agents/domain.md`.
