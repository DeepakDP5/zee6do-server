# ADR-004: Google Wire for Dependency Injection

## Status
Accepted

## Context
The modular monolith has many interdependent modules (auth, tasks, ai, scheduler, connectors, etc.) that need to be wired together. Options considered:
1. Manual constructor chaining in `main()`
2. Google Wire (compile-time code generation)
3. Uber Fx (runtime DI with lifecycle management)
4. No DI framework (manual + interfaces only)

## Decision
**Google Wire** for compile-time dependency injection.

## Consequences
- **Compile-time safety**: Wire generates the dependency graph at compile time. Missing or circular dependencies are caught before the binary runs, not at startup.
- **No reflection**: Unlike Fx, Wire does not use reflection or runtime registration. The generated code is plain Go function calls.
- **Transparency**: The generated `wire_gen.go` file shows exactly how dependencies are wired — no magic.
- **Trade-off**: Wire requires provider functions and injector declarations. This is more boilerplate than Fx's `fx.Provide()` style but more explicit.
- **Trade-off**: Lifecycle management (startup/shutdown hooks) must be handled manually, unlike Fx which provides `OnStart`/`OnStop` hooks. We handle this with our own shutdown orchestrator.
