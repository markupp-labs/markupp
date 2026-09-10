# Domain Docs

How the engineering skills should consume this repo's domain documentation when
exploring the codebase.

## Before exploring, read these

- **`CONTEXT-MAP.md`** at the repo root. It points at one `CONTEXT.md` per context.
  Read each one relevant to the topic.
- **`docs/adrs/`** for system-wide decisions. Note the directory is `adrs`, plural.
  Read the ADRs that touch the area you are about to work in.
- **`<context>/docs/adrs/`** for decisions scoped to a single context, when present.

If any of these files do not exist, proceed silently. Do not flag their absence and do
not suggest creating them upfront. The `/domain-modeling` skill creates them lazily when
terms or decisions actually get resolved.

## File structure

This repo holds one module: `markupp/`, the Go server. The Obsidian plugin lives in
`markupp-labs/obsidian-markupp-plugin`.

    /
    |- CONTEXT-MAP.md
    |- docs/adrs/                       system-wide decisions, ADR-0001 onwards
    `- markupp/
        |- CONTEXT.md
        `- docs/adrs/                   server-specific decisions

Existing ADRs are named `ADR-NNNN-slug-em-portugues.md` and are written in Portuguese.
Keep that convention for new ones.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a
hypothesis, a test name), use the term as defined in the relevant `CONTEXT.md`. Do not
drift to synonyms the glossary explicitly avoids.

If the concept you need is not in the glossary yet, that is a signal: either you are
inventing language the project does not use (reconsider) or there is a real gap (note it
for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently
overriding:

> Contradicts ADR-0013 (organizacao derivada), but worth reopening because...
