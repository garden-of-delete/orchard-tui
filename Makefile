VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/orchard-tui .

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/orchard-tui-linux-amd64 .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/orchard-tui-linux-arm64 .

.PHONY: run
run:
	go run . $(ARGS)

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	gofmt -l . | tee /tmp/orchard-tui.gofmt && test ! -s /tmp/orchard-tui.gofmt
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" .
