# Postman Assets Guide

## Implemented
- Added `currency-hedge-calculator.postman_collection.json` with all public APIs.
- Added request variants per API area:
  - plain template requests for user-provided input
  - seeded demo runs against `/v1/exposure/calculate/test`
  - production live-simulation runs against `/v1/exposure/calculate`
- Added dedicated production live-simulation requests for 50/100/1000 transactions.
- Added Postman environment file `currency-hedge-calculator.postman_environment.json`.

## Pending / Future Improvements
- Add Newman CI job to execute collection smoke tests automatically.
- Add negative-path requests for rate limiting and timeout behavior.
- Add auth key rotation workflow examples.

## Notes For Future Agents
- Keep collection variable names stable (`base_url`, `base_url_prod`, `api_key`, `idempotency_key`) to avoid breaking existing runners.
- If API schema changes, update `docs/openapi.yaml` and Postman collection in the same change.
