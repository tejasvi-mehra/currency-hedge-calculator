# AGENT_GUIDE

## Implemented
- Live FX provider against `open.er-api.com`.
- Currency whitelist validation.
- Retry handling with backoff.
- TTL cache + stale fallback behavior.
- Unit tests for retry, stale fallback, unsupported currencies.

## Pending
- Historical FX lookup by timestamp (not required for MVP).

## Key Interfaces
- `Provider`
- `Quote`
- `MemoryCache`

## Decisions and Trade-offs
- Chose public unauthenticated FX API for reviewer-friendly setup.

## Next Agent Checklist
- If switching provider, keep quote semantics and fallback behavior stable.
