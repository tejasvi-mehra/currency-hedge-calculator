# AGENT_GUIDE

## Implemented
- Service modules split into `exposure`, `rates`, and `transactions`.

## Pending
- Stretch-goal service modules (alerts/time-series) not implemented.

## Key Interfaces
- Service interfaces are declared in submodules.

## Decisions and Trade-offs
- Organized by business capability for easier ownership and testing.

## Next Agent Checklist
- Keep domain logic in services, not in framework or handlers.
