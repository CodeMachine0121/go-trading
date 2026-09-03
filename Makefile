.PHONY: start build test test-storage mock migrate

start:
	go run ./cmd/server

migrate:
	go run ./cmd/migrate

build:
	go build -o bin/server ./cmd/server
	go build -o bin/migrate ./cmd/migrate

# -race is not optional here: this codebase runs goroutines that outlive a request
# (live follows, background jobs), and a race that only CI sees is a race nobody
# fixes until it is already merged.
test:
	go test ./... -race

# Same suite, but the storage tests need TEST_POSTGRES_DSN pointing at a
# reachable PostgreSQL; without it they skip. See the README.
test-storage:
	go test ./... -count=1 -race

# Regenerates every mock. Each interface file in internal/domain/interface/
# carries its own directive:
#   //go:generate go tool mockgen -source=i_xxx.go -destination=mocks/mock_i_xxx.go -package=mocks
mock:
	go generate ./...
