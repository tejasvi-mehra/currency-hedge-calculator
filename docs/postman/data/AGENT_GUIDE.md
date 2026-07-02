# Postman Data Guide

## Implemented
- Added live simulation request payloads:
  - `live_sample_50_request.json`
  - `live_sample_100_request.json`
  - `live_sample_1000_request.json`
- Added templates:
  - `calculate_empty_template.json`
  - `demo_seeded_template.json`
- Data is diversified across:
  - 10 countries
  - 6 providers
  - 5 payment method types
  - 10 currency pairs
  - 30-day authorization spread

## Pending / Future Improvements
- Add generation script snapshot to simplify deterministic regeneration.
- Add regional stress scenarios with explicit volatility windows.

## Notes For Future Agents
- Keep payload schema aligned with `PendingTransaction` in `internal/service/exposure/types.go`.
- Preserve month-spread diversity requirement when regenerating samples.
