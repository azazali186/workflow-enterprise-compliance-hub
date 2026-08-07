.PHONY: run build test vet lint migrate-up seed permissions proto-gen tidy docker-up docker-down

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test ./... -v -cover

vet:
	go vet ./...

lint:
	golangci-lint run

migrate-up:
	go run cmd/migrate/main.go

seed:
	go run cmd/seed/main.go

permissions:
	go run cmd/permissions/main.go

proto-gen:
	@echo "gRPC definitions will be added in a later phase (see README)"

tidy:
	go mod tidy

docker-up:
	docker compose up -d postgres redis nats

docker-down:
	docker compose down
