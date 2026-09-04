# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues on `markupp-labs/markupp`,
the `origin` remote. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for
  multi-line bodies. Follow the templates in `.github/ISSUE_TEMPLATE/`.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by `jq` and
  also fetching labels.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments
  --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`
  with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

`gh` infers the repo from `git remote -v` inside the clone. This repo has three remotes
(`origin`, `upstream`, `fork`); `origin` is the one that holds the issues. Pass
`-R markupp-labs/markupp` if a command ever resolves elsewhere.

Issue bodies are written in Portuguese, like the rest of the project's documentation.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## When a skill says "publish to the issue tracker"

Create a GitHub issue against `markupp-labs/markupp`, following the matching template in
`.github/ISSUE_TEMPLATE/`.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far /
  Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (`gh api` on the
  sub-issues endpoint). Where sub-issues are not enabled, add the child to a task list in
  the map body and put `Part of #<map>` at the top of the child body. Labels:
  `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket
  is assigned to the driving dev.
- **Blocking**: GitHub's native issue dependencies, the canonical UI-visible
  representation. Add an edge with `gh api --method POST
  repos/markupp-labs/markupp/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`,
  where `<blocker-db-id>` is the blocker's numeric database id
  (`gh api repos/markupp-labs/markupp/issues/<n> --jq .id`, not the `#number` or
  `node_id`). GitHub reports `issue_dependencies_summary.blocked_by`, counting open
  blockers only, which is the live gate. Where dependencies are not available, fall back
  to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked
  when every blocker is closed.
- **Frontier query**: list the map's open children (`gh issue list --state open`, scoped to
  the map's sub-issues or task list), drop any with an open blocker
  (`issue_dependencies_summary.blocked_by > 0`, or an open issue in the `Blocked by` line)
  or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me`, the session's first write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then `gh issue close <n>`, then
  append a context pointer (gist plus link) to the map's Decisions-so-far.
