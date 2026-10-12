# Testing

DataDeck uses Go tests for the backend, Vitest + React Testing Library for the
frontend, and Playwright for browser end-to-end tests.

## Backend

```bash
cd backend
go test ./...                       # unit tests
go test -race ./...                 # race detector
go vet ./...                        # static checks
gofmt -l .                          # formatting (expect no output)
```

### Integration tests (PostgreSQL / MySQL)

Integration tests are behind the `integration` build tag and require reachable
servers plus a **disposable** test database:

```bash
cd backend
DATADECK_TEST_PG_HOST=127.0.0.1 DATADECK_TEST_PG_USER=<user> \
DATADECK_TEST_MYSQL_HOST=127.0.0.1 \
  go test -tags=integration -p 1 ./...
```

`-p 1` serializes packages because the database integration tests share a server
and some create/drop databases.

## Frontend

```bash
cd frontend
npm run lint        # eslint
npm run typecheck   # tsc --noEmit
npm run test        # vitest run
npm run build       # production build
npm run build:static# static export (embedded into the binary)
```

## Browser E2E (Playwright)

E2E runs against an already-started single binary (no `webServer` in the config):

```bash
# 1. build and start the embedded binary (see building.md), listening on :8099
# 2. run the suite
cd frontend
E2E_BASE_URL=http://127.0.0.1:8099 E2E_PG_HOST=127.0.0.1 npx playwright test
```

Specs use **disposable** databases created through the API and assert real backend
state; they are skipped when `E2E_PG_HOST` is unset.

## OpenAPI

The committed spec is generated from handler annotations:

```bash
cd backend && make swagger        # regenerates backend/docs/swagger.json|yaml
cd frontend && npm run generate:api  # regenerates src/types/generated/openapi.ts
```

`internal/api/contract_test.go` asserts the committed spec matches the router.

## Version consistency

```bash
./scripts/check-version.sh        # VERSION == version.go == package(s) == Dockerfile
```
