# Database Specs

> Spec-first migration process. **No new entity / table / major schema change without an approved spec in this directory first.**

## Why

`db/migrations/` is the source of truth for what schema **is**. This directory is the source of truth for **why** it's that way. Without specs:
- Designers make conflicting assumptions
- Reviewers can't catch bad model decisions early
- Future maintainers can't tell intentional choices from accidents

## Process

1. **Author** copies `_template.md` → `<entity>.md`
2. **Author** fills in all 8 sections (no "TBD" — if unknown, flag in section 7)
3. **Reviewer** reviews via PR
4. **Project lead** sign-off → status changes to "Approved"
5. **Implementation** PR can now write migrations + handlers + UI
6. Spec status updated to "Implemented" when shipped

## Status definitions

| Status | Meaning |
|---|---|
| Draft | Author still iterating. Don't review yet. |
| Reviewed | Reviewer comments addressed. Awaiting project lead. |
| Approved | Spec is contract. Implementation can begin. |
| Implemented | Code shipped. Spec frozen as historical record. |
| Deprecated | Entity removed or significantly redesigned. New spec supersedes. |

## Naming convention

`<lowercase-entity>.md` for new entities.
`<lowercase-domain>-<change>.md` for major changes to existing entities.

Examples:
- `services.md`
- `incidents.md`
- `assets-soft-delete.md`

## When NOT to write a spec

- Adding a single column with default value (just write migration)
- Adding an index (write migration with comment)
- Renaming a field (cutover SQL is enough)
- Bug fix migrations

The bar: **if it changes how someone would think about the data model, write a spec**.
