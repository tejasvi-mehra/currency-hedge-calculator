# AGENT_GUIDE

## Implemented
- Zap logger builder with env-driven mode/level/encoding.

## Pending
- None.

## Key Interfaces
- `Build(Config) (*zap.SugaredLogger, error)`

## Decisions and Trade-offs
- Sugared logger selected for speed of implementation and readability.

## Next Agent Checklist
- Preserve structured logging fields for observability.
