# AGENT_GUIDE

## Implemented
- File-backed in-memory pending transaction store.

## Pending
- DB-backed repository is not implemented (MVP intentionally in-memory).

## Key Interfaces
- `FileStore.ListPending(...)`

## Decisions and Trade-offs
- Startup file load is sufficient for deterministic demos and analytics replay.

## Next Agent Checklist
- Keep returned slices copied to avoid shared mutable state bugs.
