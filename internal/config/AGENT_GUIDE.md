# AGENT_GUIDE

## Implemented
- Environment-driven config model in `config.go`.
- Validation and normalization for server/FX/data settings.

## Pending
- None for MVP.

## Key Interfaces
- `config.Config` root runtime configuration.

## Decisions and Trade-offs
- Chose env-first config with safe defaults for local/demo deployment speed.

## Next Agent Checklist
- Keep new env vars documented in `.env.example` and `README.md`.
