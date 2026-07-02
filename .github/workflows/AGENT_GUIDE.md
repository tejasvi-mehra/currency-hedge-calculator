# AGENT_GUIDE

## Implemented
- `unit-tests.yml` workflow runs `go test ./...` on PRs and pushes.

## Pending
- No other workflows required for MVP.

## Key Interfaces
- GitHub Actions `go-test` job.

## Decisions and Trade-offs
- Minimal workflow to satisfy mandatory pre-merge test check.

## Next Agent Checklist
- Keep Go version sourcing from `go.mod` for consistency.
