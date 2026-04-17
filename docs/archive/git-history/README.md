# Pre-Monorepo Git History Archive

Before `/cmdb-platform` became a monorepo, the three sub-projects
(`cmdb-core`, `cmdb-demo`, `ingestion-engine`) were developed as
independent git repositories. When they were consolidated into the
current monorepo, each sub-project's `.git/` directory was left in
place. These archives preserve that orphan history for
forensic / audit purposes only.

Archived on: 2026-04-17

## Contents

| Archive | Commits | HEAD | Archive size |
|---------|---------|------|--------------|
| `cmdb-core-dot-git.tar.gz`       | 41 | `77c044c` — feat: add location anomaly detection and governance report endpoints | 624K |
| `cmdb-demo-dot-git.tar.gz`       | 11 | `7a256e1` — feat(i18n): extract hardcoded strings from 6 frontend files into translation keys | 2.2M |
| `ingestion-engine-dot-git.tar.gz` |  9 | `41e5559` — feat: add ingestion-engine to docker-compose + local dev runner | 48K |

Each archive contains a tag `archive/pre-monorepo-20260417` pointing
at the HEAD at the moment of archival.

## Why archived rather than merged into the main repo?

The main monorepo already contains richer history for each sub-directory:

| Sub-project | Main-repo commits touching this path | Archived-repo commits |
|-------------|--------------------------------------|-----------------------|
| `cmdb-core/`        | **152** | 41 |
| `cmdb-demo/`        | **226** | 11 |
| `ingestion-engine/` |  **26** |  9 |

The two histories share **no** commit hashes — they are fully parallel
timelines. Merging them via `git subtree` or `--allow-unrelated-histories`
would produce a tangled graph with no real benefit, since the
monorepo's own history is already more complete.

## Restoration

If you ever need to inspect an archived history:

```bash
mkdir /tmp/recover && cd /tmp/recover
tar xzf /path/to/docs/archive/git-history/cmdb-core-dot-git.tar.gz
git --git-dir=.git log --all --oneline
git --git-dir=.git show <commit-hash>
```

Do **not** restore these archives back under `/cmdb-platform/<sub>/.git`
— that would recreate the nested-git problem that archiving solved.

## Note on `ingestion-engine`

The pre-gc size of `ingestion-engine/.git` was **51 MB**, due to a
dangling "initial state" commit (`c9298248`, 2026-04-03) that had
erroneously committed the entire `.venv/` virtualenv (including ELF
`.so` compiled extensions). That commit was already orphaned before
archival. We ran `git gc --prune=now --aggressive` before archiving,
dropping the size to 216 KB (2.8 MB tarball → 48 KB compressed). All
9 reachable commits are preserved; only the erroneous `.venv` objects
were dropped.
