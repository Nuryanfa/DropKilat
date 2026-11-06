# ADR 0001 — Modular monolith in Go with PostgreSQL as the only stateful service

Status: accepted · Date: 2026-11-06

## Context
One developer team (one human, three AI agents), one 2 vCPU / 4 GB VM, a core problem that is a concurrency problem on stock, and a need to deploy and debug cheaply.

## Decision
One Go binary containing the HTTP server, SSE and background workers. PostgreSQL 17 is the only stateful dependency: stock, queues (outbox, webhook events), and real-time fan-out (`LISTEN/NOTIFY`). Server-rendered HTML (templ + htmx) instead of a separate SPA.

## Consequences
- One deploy unit, one log stream, transactions across modules are possible.
- Hot-row contention on a popular variant is the known limit; mitigation (stock sharding) is deferred until load tests show it is needed. Redis is not added for the MVP.
- If a richer client is needed later, the JSON API under `/api` already exists.
