# AGENT_GUIDE

## Implemented
- Exponential retry strategy and context-aware sleep helper.

## Pending
- No jitter strategy yet (optional enhancement).

## Key Interfaces
- `Strategy`

## Decisions and Trade-offs
- Deterministic exponential backoff chosen for simplicity.

## Next Agent Checklist
- Add jitter only if external APIs become noisier under load.
