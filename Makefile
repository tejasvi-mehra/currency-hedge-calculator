.PHONY: run run-env analytics-local test fmt tidy docker-build docker-up docker-down

run:
	go run .

run-env:
	@set -a; [ -f .env ] && . ./.env; set +a; go run .

analytics-local:
	@curl --request POST \
		--url $${API_URL:-http://localhost:8080}/v1/exposure/calculate/test \
		--header 'Content-Type: application/json' \
		--header 'X-API-Key: '$${API_AUTH_KEY:-dev-api-key} \
		--header 'X-Idempotency-Key: 11111111-1111-4111-8111-111111111111' \
		--data '{"risk_threshold_percentage":2}'

test:
	go test ./...

fmt:
	gofmt -w $$(rg --files -g "*.go")

tidy:
	go mod tidy

docker-build:
	docker build -t currency-hedge-calculator:local .

docker-up:
	docker compose up --build

docker-down:
	docker compose down
