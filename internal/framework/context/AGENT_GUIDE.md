# AGENT_GUIDE

## Implemented
- Signal-based graceful shutdown context helper.

## Pending
- None.

## Key Interfaces
- `WithShutdownSignals(parent context.Context)`

## Decisions and Trade-offs
- Minimal wrapper to keep lifecycle logic explicit and testable.

## Next Agent Checklist
- Reuse this helper for new long-running processes.
