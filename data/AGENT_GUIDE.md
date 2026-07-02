# AGENT_GUIDE

## Implemented
- Seed dataset with 50 pending transactions for exposure demos/tests.

## Pending
- Optional generator script not included to keep MVP lean.

## Key Interfaces
- `data/analytics_test_transactions.json` consumed by `transactions.FileStore`.

## Decisions and Trade-offs
- Stored deterministic sample data directly in repo for easy reviewer execution.

## Next Agent Checklist
- Preserve currency distribution constraints when modifying analytics test data.
