# AGENT_GUIDE

## Implemented
- Framework modules for logger, server, context signals, HTTP connector, backoff, and runner.

## Pending
- DB/Redis middleware intentionally not implemented for MVP scope.

## Key Interfaces
- `server.Context`
- `backoff.Strategy`
- `runner.Runnable`

## Decisions and Trade-offs
- Used only framework pieces required by challenge acceptance criteria.

## Next Agent Checklist
- Keep framework packages business-agnostic.
