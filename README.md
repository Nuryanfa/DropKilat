# DropKilat

> Drop habis dalam menit. Tanpa rekap manual, tanpa oversell, tanpa pembeli kabur.

DropKilat is a checkout link for UMKM sellers who run limited **drops** and **pre-orders (PO)** on WhatsApp, Instagram and TikTok. A buyer who clicks **Ambil** gets the item locked for 15 minutes while they pay. If they don't pay, the item moves automatically to the next person in the waiting list.

Status: **pre-MVP**. Docs are the source of truth; code follows them.

## Documents

| Doc | What it answers |
| --- | --- |
| [docs/prd.md](docs/prd.md) | What we build, for whom, and how we know it works |
| [docs/design.md](docs/design.md) | Screens, components, copy, visual tokens |
| [docs/trd.md](docs/trd.md) | Architecture, stack, non-functional requirements, security |
| [docs/app-flow.md](docs/app-flow.md) | User flows and state machines |
| [docs/backend.md](docs/backend.md) | Modules, HTTP API, jobs, errors, config |
| [docs/database-schema.md](docs/database-schema.md) | Tables, constraints, indexes, RLS, key queries |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Phases, tasks, owners, acceptance criteria |
| [docs/adr/](docs/adr/) | Architecture decisions |
| [docs/agents/workflow.md](docs/agents/workflow.md) | How the three AI agents work together |

## AI agent team

| Agent | Role | Instruction file |
| --- | --- | --- |
| Claude | Tech lead: architecture, task specs, review, merge | [CLAUDE.md](CLAUDE.md) |
| Codex | Full-stack engineer: implements tasks | [AGENTS.md](AGENTS.md) |
| Gemini | QA engineer: test plans, tests, bug reports, security checks | [GEMINI.md](GEMINI.md) |

The human owner (Nur Yanfa) is the product owner and has the final say on scope, money and anything irreversible.

## Quick start (once code exists)

```bash
cp .env.example .env
make db-up        # PostgreSQL in Docker
make migrate      # apply migrations/
make run          # http://localhost:8080
make test         # unit + integration
```

## Repository layout

```
cmd/dropkilat/        main binary (HTTP server + background workers)
internal/
  drop/               drops and variants (seller side)
  order/              claim, hold, expiry, order state machine
  waitlist/           queue and promotion
  payment/            providers: manual, xendit, fake
  buyer/              buyer identity and WhatsApp verification
  seller/             seller accounts, sessions, settings
  trust/              verification levels, track record, reports
  notify/             outbox and WhatsApp sender
  platform/           db, http helpers, config, clock
web/                  templ templates and static assets
migrations/           SQL migrations (golang-migrate format)
tests/                integration, load (k6), e2e (Playwright)
prototype/            clickable HTML prototype for seller interviews
deploy/               Docker Compose (dev now; VM + Caddy in T-703)
scripts/              helper scripts (migrate-check)
docs/                 product and technical docs
```
