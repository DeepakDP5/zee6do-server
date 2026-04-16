# Zee6do Beta-Ready Checklist — Reference

> **This is a reference pointer.** The canonical, checkable checklist lives at:
>
> `/memory/knowledge/checklists/beta-checklist.md`
>
> That file is the single source of truth. All sessions read from and write to that location.

## What Is This?

A comprehensive beta-ready checklist with **773 checkable items** covering:

- **Server (Sections 1–20):** Foundation, database, gRPC/proto, auth, users, tasks, AI orchestration, scheduler, connectors, knowledge base, notifications, real-time, analytics, sync, billing, storage, feature flags, cross-cutting concerns, DevOps, testing.
- **Client (Sections 21–42):** Foundation, design system, navigation, state management, WatermelonDB, API layer, auth/onboarding, dashboard, task management, quick add, AI assistant, analytics, profile/settings, connectors UI, knowledge base UI, search, focus mode, notifications, offline sync, platform features, cross-cutting, testing.
- **Integration (Sections 43–44):** End-to-end flows, beta launch readiness.
- **Progress Log (Section 45):** Dated entries per session.

## Scope Key

- Items with no tag = **Required for Beta** (~1,000 user closed beta)
- Items tagged **[Post-Beta]** = Future phases, not blocking beta launch

## How to Update

Every session that makes implementation progress should:

1. Open `/memory/knowledge/checklists/beta-checklist.md`
2. Change `- [ ]` to `- [x]` for completed items
3. Add a dated entry to the Progress Log (Section 45)
