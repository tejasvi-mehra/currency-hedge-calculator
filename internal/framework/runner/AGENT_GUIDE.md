# AGENT_GUIDE

## Implemented
- Minimal parallel runnable coordinator using `errgroup`.

## Pending
- Not currently used by `App`, kept available for future background stages.

## Key Interfaces
- `Runnable`
- `RunParallel(ctx, runnables...)`

## Decisions and Trade-offs
- Added lightweight runner instead of heavier hook-based lifecycle system.

## Next Agent Checklist
- Use runner when adding concurrent workers beyond HTTP server lifecycle.
