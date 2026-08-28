.PHONY: start build test mock

start:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

# Regenerates every mock. Each interface file in internal/domain/interface/
# carries its own directive:
#   //go:generate go tool mockgen -source=i_xxx.go -destination=mocks/mock_i_xxx.go -package=mocks
mock:
	go generate ./...
