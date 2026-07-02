.PHONY: run run-env test fmt tidy docker-build docker-up docker-down

run:
	go run .

run-env:
	@set -a; [ -f .env ] && . ./.env; set +a; go run .

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
