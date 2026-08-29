.PHONY: start build test mock migrate

start:
	go run ./cmd/server

migrate:
	go run ./cmd/migrate

build:
	go build -o bin/server ./cmd/server
	go build -o bin/migrate ./cmd/migrate

test:
	go test ./...

# Regenerates every mock. Each interface file in internal/domain/interface/
# carries its own directive:
#   //go:generate go tool mockgen -source=i_xxx.go -destination=mocks/mock_i_xxx.go -package=mocks
mock:
	go generate ./...
