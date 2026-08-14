# Agent Guidelines

## Scope

- Keep ID Ledger lightweight: one service instance, one SQLite writer.
- Store IDs as case-sensitive strings scoped by a stable project key.
- Set `created_at` only on the server when an ID is first persisted.
- Preserve batch-write idempotency and stable time-based pagination.
- Do not add leases, schedulers, distributed locks, or per-project tables without an explicit requirement.

## Toolchain

- Go with Fiber v3, SQLite, sqlc, and Viper.
- Format Go changes with `gofmt` and verify them with `go test ./...`.
- Keep secrets out of source control; configuration belongs in environment variables.
- Prefer the standard library and existing dependencies over new abstractions.

## Changes

- Make the smallest complete change and include a focused test for non-trivial logic.
- Keep README, AI Skill, query scripts, container configuration, and CI usage accurate when their behavior changes.
