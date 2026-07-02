# AGENT_GUIDE

## Implemented
- Go backend MVP for currency exposure calculation.
- Live FX integration, risk ranking, tests, Docker/CI, and OpenAPI docs.

## Pending
- No blocker for MVP completion.
- Optional future work is listed in `README.md`.

## Key Interfaces
- `exposure.TransactionSource`
- `rates.Provider`
- `framework/server.Context`

## Decisions and Trade-offs
- Focused on one core endpoint first for predictable delivery.
- Used default test data + in-memory loading instead of DB persistence.

## Next Agent Checklist
- Run `go test ./...`.
- Validate live FX API availability in your environment.
- Keep API field naming as `snake_case`.
