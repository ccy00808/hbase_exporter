BINARY   := hbase_exporter
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS  := -X github.com/prometheus/common/version.Version=$(VERSION) \
            -X github.com/prometheus/common/version.Revision=$(COMMIT) \
            -X github.com/prometheus/common/version.Branch=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GOFLAGS  := -trimpath -ldflags "$(LDFLAGS)"

.PHONY: build test vet fmt clean docker

build:
	go build $(GOFLAGS) -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

docker:
	docker build -t $(BINARY):$(VERSION) .

clean:
	rm -f $(BINARY)
	rm -rf dist/
