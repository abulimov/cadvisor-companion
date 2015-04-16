.PHONY: all test clean build docker publish

GOFLAGS ?= $(GOFLAGS:)
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0
REGISTRY ?= abulimov
VERSION = `grep 'version =' cadvisor-companion.go | sed 's/[a-z "=]//g'`

all: test

build:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) github.com/abulimov/cadvisor-companion

test:
	@go test $(GOFLAGS) github.com/abulimov/cadvisor-companion/process

clean:
	@go clean $(GOFLAGS) -i github.com/abulimov/cadvisor-companion

docker: build
	@docker build -t $(REGISTRY)/cadvisor-companion:$(VERSION) .

publish: docker
	@docker tag -f $(REGISTRY)/cadvisor-companion:$(VERSION) $(REGISTRY)/cadvisor-companion:latest
	@docker push $(REGISTRY)/cadvisor-companion:$(VERSION)
	@docker push $(REGISTRY)/cadvisor-companion:latest
