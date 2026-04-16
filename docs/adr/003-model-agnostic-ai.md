# ADR-003: Model-Agnostic AI Orchestration

## Status
Accepted

## Context
The AI capabilities of Zee6do are central to the product — task lifecycle management, smart scheduling, NLP parsing, analytics narratives, and more. The LLM landscape is evolving rapidly with frequent model releases, pricing changes, and capability shifts across OpenAI, Anthropic, Google, and open-source alternatives.

Tying the server to a single LLM provider creates vendor lock-in and prevents us from choosing the best model for each task type.

## Decision
**Model-agnostic LLM provider interface** in the `ai` module:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

Implementations exist for each provider (OpenAI, Anthropic, Google). The active provider is selected via configuration. Different operations can use different providers if needed (e.g., GPT-4o for NLP parsing, Claude for analytics narratives).

## Consequences
- Switching providers requires a config change, not a code change.
- Different AI operations can use different providers optimized for cost/quality.
- The abstraction adds a thin layer of indirection. Provider-specific features (tool calling syntax, system prompt conventions) are normalized in each adapter.
- New providers can be added by implementing the interface — no changes to service or handler code.
