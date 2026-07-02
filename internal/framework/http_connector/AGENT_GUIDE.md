# AGENT_GUIDE

## Implemented
- Reusable outbound HTTP GET JSON connector.
- Standardized non-2xx error wrapping (`HTTPError`).

## Pending
- POST/PUT helpers are not implemented because not needed by MVP.

## Key Interfaces
- `Connector.GetJSON(...)`
- `HTTPError`

## Decisions and Trade-offs
- Kept connector narrow to reduce complexity and bugs.

## Next Agent Checklist
- Extend connector only when a second use case requires it.
