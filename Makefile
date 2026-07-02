.PHONY: run run-env analytics-local test fmt tidy docker-build docker-up docker-down

run:
	go run .

run-env:
	@set -a; [ -f .env ] && . ./.env; set +a; go run .

analytics-local:
	@python3 -c "import json,sys; print(json.dumps({'risk_threshold_percentage': 2, 'transactions': json.load(open('data/analytics_test_transactions.json'))}))" | curl --request POST \
		--url $${API_URL:-http://localhost:8080}/v1/exposure/calculate \
		--header 'Content-Type: application/json' \
		--data @-

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
