# AGENT_GUIDE

## Implemented
- Request/response contracts (`types.go`).
- Exposure calculation and ranking logic (`service.go`).
- HTTP handler and error mapping (`handler.go`).
- Unit tests for service math and handler behavior.

## Pending
- Optional advanced risk heuristics (trend-based recommendations).

## Key Interfaces
- `Service.CalculateExposure(...)`
- `TransactionSource`
- `Clock`

## Decisions and Trade-offs
- Ranking currently sorts by lowest exposure amount first (worst loss first).

## Next Agent Checklist
- Keep rate convention unchanged unless all docs/tests are updated.
