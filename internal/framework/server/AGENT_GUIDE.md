# AGENT_GUIDE

## Implemented
- Echo-backed framework server wrapper with:
  - recover middleware
  - request ID middleware
  - structured request logging
  - CORS
  - GET/POST registration abstractions

## Pending
- Auth middleware is intentionally out of MVP scope.

## Key Interfaces
- `Server`
- `Context`
- `HandlerFunc`

## Decisions and Trade-offs
- Kept a thin adapter to avoid tight coupling to Echo in business handlers.

## Next Agent Checklist
- Preserve error envelope behavior from handler layer.
