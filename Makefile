BINARY    := giraffemail
CMD       := ./cmd/giraffemail
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
LDFLAGS   := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-linux build-darwin-arm64 build-darwin-amd64 build-windows \
        build-ui test test-ui test-cover clean run docker-build docker-up

## Build for the current platform
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) $(CMD)

## Cross-compile for Linux amd64
build-linux:
	GOOS=linux   GOARCH=amd64  CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-linux-amd64  $(CMD)

## Cross-compile for macOS Apple Silicon
build-darwin-arm64:
	GOOS=darwin  GOARCH=arm64  CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 $(CMD)

## Cross-compile for macOS Intel
build-darwin-amd64:
	GOOS=darwin  GOARCH=amd64  CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 $(CMD)

## Cross-compile for Windows amd64
build-windows:
	GOOS=windows GOARCH=amd64  CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe $(CMD)

## Build all release targets
release: build-linux build-darwin-arm64 build-darwin-amd64 build-windows

## Build the Next.js static export and copy it into internal/ui/dist/
build-ui:
	cd frontend && pnpm install && NEXT_OUTPUT=export pnpm build
	rm -rf internal/ui/dist
	cp -r frontend/out internal/ui/dist

## Build the Go binary (run build-ui first for the embedded UI)
build-full: build-ui build

## Run tests
test:
	go test ./... -v -race

## Frontend unit tests
test-ui:
	cd frontend && pnpm test run

## Run tests with coverage report
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

## Remove build artifacts
clean:
	rm -f $(BINARY) $(BINARY)-* coverage.out coverage.html

## Run the server in dev mode (requires config.yaml)
run: build
	./$(BINARY) serve --config config.yaml

## Build Docker image (requires Docker)
docker-build:
	docker build -t giraffemail:latest .

## Start via docker compose (set GM_SECRET_KEY in .env)
docker-up:
	docker compose up -d --build
