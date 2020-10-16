GITCOMMIT := $(shell git describe --always)
VERSION := $(shell git describe --tags 2>/dev/null || echo "latest")
PROJECTNAME := $(shell basename "$(PWD)")

GOBASE := $(shell pwd)
GOPATH := $(GOBASE)/vendor:$(GOBASE)
GOBIN := $(GOBASE)/bin
GOFILES := $(wildcard *.go)

LDFLAGS=-ldflags="-X 'github.com/kronostechnologies/richman/cmd.Version=$(VERSION)' -X 'github.com/kronostechnologies/richman/cmd.GitCommit=$(GITCOMMIT)' $(EXTRA_LDFLAGS)"

.PHONY: all
all: setup check test compile package

.PHONY: setup
setup: check
	@echo ">>> Fetching dependencies..."
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go get

.PHONY: check
check:
	@echo ">>> Checking go..."
	@command -v go >/dev/null 2>&1 || { echo "go is not installed. Install it https://golang.org/doc/install."; exit 1; }

.PHONY: test
test: setup
	@echo ">>> Running Test Suite..."
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test -v ./...

.PHONY: compile
compile: setup
	@echo ">>> Compiling..."
	GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build $(LDFLAGS) -o $(GOBIN)/$(PROJECTNAME) $(GOFILES)
	@echo ">>> Source available at $(GOBIN)/$(PROJECTNAME)"

clean:
	@echo ">>> Cleaning..."
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go clean
	@rm -rf $(GOBIN)

.PHONY: package
package: package.image

.PHONY: package.image
package.image:
	@echo ">>> Building docker image $(PROJECTNAME):$(VERSION)"
	docker build . -t $(PROJECTNAME):$(VERSION)

