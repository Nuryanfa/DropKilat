# Worktrees and agent monitoring

Each agent works in its own git worktree so that no two agents ever edit the same working copy. Claude (tech lead) owns these rules and checks compliance with `scripts/agents-status.sh` at the start of every session and before every review.

## Layout

```
C:/laragon/www/dropkilat/
  dropkilat/                 main checkout: Claude (docs, task cards, review, merge)
  wt/
    codex-T-001/             Codex, branch feat/T-001-<slug>
    gemini-T-001/            Gemini, branch test/T-001-<slug> (or detached for review only)
```

- One worktree = one agent + one task. Name: `wt/<agent>-<task-id>`.
- Create worktrees only with `scripts/wt-new.sh`; it also writes the worktree's own `.env`.
- Never open, edit, `git reset`, `git clean` or remove another agent's worktree.
- After merge Claude removes the worktree: `git worktree remove ../wt/<name>` and deletes the branch.

## Branches

| Agent | Branch | Based on |
| --- | --- | --- |
| Claude | `docs/<slug>` | `origin/main` |
| Codex | `feat/T-xxx-<slug>`, `fix/BUG-xxx-<slug>` | `origin/main` |
| Gemini | `test/T-xxx-<slug>` | Codex's `feat/T-xxx-…` |

Git does not allow the same branch to be checked out in two worktrees. Gemini therefore never checks out Codex's branch directly: it branches `test/T-xxx-…` from it, and the PR for those tests targets `feat/T-xxx-…` (or `main` after the feature merges). Codex merges Gemini's test PR into its branch; nobody pushes to another agent's branch.

Rules for every agent:

- `git fetch origin` and rebase on `origin/main` before opening a PR and before asking for review.
- Never force-push a branch that another agent has based work on (`feat/…` once Gemini has started). Use `--force-with-lease` only on your own unshared branch.
- Never push to `main`. Never merge your own PR. Only Claude merges.
- Commit at least once per working session with a conventional commit message, so progress is visible to `agents-status.sh`. Uncommitted work is invisible to everyone else.

## File ownership

`agents-status.sh` flags changes outside an agent's area. A flag is not automatically wrong, but it must be explained in the PR description or the card.

| Area | Codex | Gemini | Claude |
| --- | --- | --- | --- |
| `cmd/`, `internal/` (non-test), `web/`, `migrations/`, `prototype/`, `go.mod` | writes | read only | review fixes only |
| `*_test.go`, `tests/` | writes | writes | review fixes only |
| `docs/test-plan.md`, `docs/tasks/*-qa.md`, `docs/tasks/BUG-*.md` | read only | writes | reads |
| `docs/*.md` (prd, trd, backend, schema, flow, design) | only what the card asks | read only | writes |
| `docs/adr/`, `docs/agents/`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `.github/`, `scripts/`, `Makefile` | read only | read only | writes |

Task cards (`docs/tasks/T-xxx.md`) are shared, but each agent edits only its own parts:

- Claude: everything except the parts below; answers under **Questions**; status `draft → ready`, `qa-pass → done`.
- Codex: adds entries under **Questions**, writes the Codex part of **Hand-off note**; status `ready → in-progress → in-review`.
- Gemini: writes the Gemini part of **Hand-off note**; status `in-review → qa-pass` (or back to `in-progress` on FAIL).
- Anyone may set `blocked` with a one-line reason under **Questions**.

## Ports and databases

Each worktree runs its own server and database so that runs and migrations never collide. `wt-new.sh` sets these in the worktree's `.env`:

| Checkout | `DK_HTTP_ADDR` | Database |
| --- | --- | --- |
| main (Claude) | `:8080` | `dropkilat` |
| `wt/codex-*` | `:8081` | `dropkilat_codex` |
| `wt/gemini-*` | `:8082` | `dropkilat_gemini` |

All share the one Postgres container from `make db-up`. Integration tests use testcontainers and need no shared state. To reset an agent's database: `make migrate-down` repeatedly, or drop and recreate it.

## Monitoring (Claude)

At the start of each session and before each review, Claude runs:

```bash
./scripts/agents-status.sh
```

It shows, for every worktree: agent, branch, last commit and its age, uncommitted files, ahead/behind `origin/main`, and files touched outside the agent's area. Then it lists the status of every task card and flags mismatches (card `in-progress` with no worktree, worktree with no card, no commit for more than 24 hours).

Claude acts on what it finds:

| Signal | Action |
| --- | --- |
| Ownership flag without explanation | Line comment on the PR, or a note under the card's **Questions** |
| Card `in-progress`, no commit for over 24 h | Ask under **Questions** whether the card is blocked; split the task if it is too big |
| Branch over 20 commits behind `main` | Ask the owner to rebase before review |
| Unanswered entry under **Questions** | Answer in the same session (rule in `CLAUDE.md`) |
| Two worktrees changing the same file | Decide which card owns it; record the decision in both cards |

Claude only reports and writes in cards. It never edits another agent's worktree to "fix" things. Fixes go through the owner, or through a review-fix commit on the PR branch after QA PASS.
