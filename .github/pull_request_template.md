## Task
T-xxx — <title> (`docs/tasks/T-xxx.md`)

## What and why

## How it was tested
- [ ] `make lint test migrate-check`
- [ ] Concurrency/E2E/load (if stock, payments or UI flows changed)

## Screenshots (UI changes)

## Docs touched
- [ ] PRD / design / TRD / app-flow / backend / schema / plan / ADR — or "none"

## Risks
Money, stock, auth, privacy, migrations:

## Checklist
- [ ] Transactions wrap every multi-row change; side effects via outbox in the same tx
- [ ] Seller queries filter by `seller_id` and use the RLS context helper
- [ ] No PII or secrets in logs
- [ ] New env vars added to `.env.example` and `docs/backend.md` §8
- [ ] QA report: `docs/tasks/T-xxx-qa.md`
