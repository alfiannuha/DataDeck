GO ?= go

.PHONY: build run test vet fmt tidy clean swagger

build:
	$(GO) build -o bin/datadeck ./cmd/server

run:
	$(GO) run ./cmd/server

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

# Regenerates docs/swagger.json and docs/swagger.yaml (OpenAPI 3.1).
swagger:
	$(GO) run github.com/swaggo/swag/v2/cmd/swag@latest init -g cmd/server/main.go -o docs --parseInternal --outputTypes json,yaml --v3.1

clean:
	rm -rf bin
