# AGENT_GUIDE

## Implemented
- Top-level internal packages split into `config`, `framework`, and `service`.

## Pending
- No required MVP work pending in this directory root.

## Key Interfaces
- Boundary ownership only; interfaces are defined in subpackages.

## Decisions and Trade-offs
- Kept infra (`framework`) separate from business logic (`service`) for testability.

## Next Agent Checklist
- Add new modules under `service` and keep reusable code under `framework`.
