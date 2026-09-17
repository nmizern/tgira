BINARY      := tgira
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.8.1

.PHONY: build test lint fmt run release clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/tgira

test:
	go test -race ./...

lint:
	go vet ./...
	go run $(STATICCHECK) ./...

fmt:
	gofmt -w .

run: build
	./$(BINARY) -config config.yaml

release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/tgira

clean:
	rm -rf $(BINARY) dist
