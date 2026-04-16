# ADR-005: Error Wrapping + Sentinel Errors

## Status
Accepted

## Context
Go's error handling requires a consistent pattern across the codebase. Options considered:
1. Standard `errors.New()` / `fmt.Errorf()` with ad-hoc patterns
2. A custom errors package with typed errors, stack traces, and gRPC status mapping
3. Error wrapping with `%w` + sentinel errors + `errors.Is/As` checking

## Decision
**Error wrapping with `fmt.Errorf("%w")` + sentinel errors for known conditions + `errors.Is/As` for checking.**

This follows idiomatic Go patterns without introducing a custom error framework.

## Consequences
- Every error is wrapped with context as it propagates up the call stack, creating a readable chain.
- Sentinel errors (`ErrTaskNotFound`, `ErrUnauthorized`, `ErrConflict`) are defined per domain for known, checkable conditions.
- gRPC status code mapping happens at the handler boundary only — service and repository code works with domain errors, not gRPC codes.
- No stack traces in errors (unlike pkg/errors). Stack traces add allocation overhead and are rarely needed when error chains have sufficient context. If debugging requires stack traces, use the debugger.
- Developers must be disciplined about wrapping — a bare `return err` without context makes debugging harder. This is enforced in code review.
