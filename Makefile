PACKAGE := github.com/urfave/gfmrun
ALL_PACKAGES := $(PACKAGE) $(PACKAGE)/cmd/...

GIT ?= git
GO ?= go
GOLANGCI_LINT?= golangci-lint
PYTHON ?= python

VERSION_VAR := $(PACKAGE).VersionString
VERSION_VALUE ?= $(shell $(GIT) describe --always --dirty --tags 2>/dev/null)
REV_VAR := $(PACKAGE).RevisionString
REV_VALUE ?= $(shell $(GIT) rev-parse HEAD 2>/dev/null || echo "'???'")
GENERATED_VAR := $(PACKAGE).GeneratedString
GENERATED_VALUE ?= $(shell $(PYTHON) ./plz date)
COPYRIGHT_VAR := $(PACKAGE).CopyrightString
COPYRIGHT_VALUE ?= $(shell $(PYTHON) ./plz copyright)

GOPATH ?= $(shell ./plz gopath)
GOBUILD_LDFLAGS ?= \
	-X '$(VERSION_VAR)=$(VERSION_VALUE)' \
	-X '$(REV_VAR)=$(REV_VALUE)' \
	-X '$(GENERATED_VAR)=$(GENERATED_VALUE)' \
	-X '$(COPYRIGHT_VAR)=$(COPYRIGHT_VALUE)'

.PHONY: all
all: clean test

.PHONY: test
test: deps build coverage.html selftest

.PHONY: test-no-cover
test-no-cover:
	$(GO) test -v -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)

.PHONY: test-race
test-race: deps
	$(GO) test -v -race -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)

.PHONY: selftest
selftest:
	$(GO) run ./cmd/gfmrun/main.go -D -c $(shell $(PYTHON) ./plz test-count README.md)

coverage.html: coverage.coverprofile
	$(GO) tool cover -html=$^ -o $@

coverage.coverprofile:
	$(GO) test -v -x -cover -coverprofile=$@ -ldflags "$(GOBUILD_LDFLAGS)" $(PACKAGE)
	$(GO) tool cover -func=$@

.PHONY: lint
lint:
	$(GOLANGCI_LINT) run

.PHONY: build
build: deps
	GOOS=linux $(GO) build -o gfmrun_linux_$(VERSION_VALUE)/gfmrun  -x -ldflags "$(GOBUILD_LDFLAGS)" ./cmd/gfmrun/main.go
	tar -czf gfmrun_linux_$(VERSION_VALUE).tar.gz gfmrun_linux_$(VERSION_VALUE)
	GOOS=darwin $(GO) build -o gfmrun_darwin_$(VERSION_VALUE)/gfmrun  -x -ldflags "$(GOBUILD_LDFLAGS)" ./cmd/gfmrun/main.go
	tar -czf gfmrun_darwin_$(VERSION_VALUE).tar.gz gfmrun_darwin_$(VERSION_VALUE)
	GOOS=windows $(GO) build -o gfmrun_windows_$(VERSION_VALUE)/gfmrun.exe  -x -ldflags "$(GOBUILD_LDFLAGS)" ./cmd/gfmrun/main.go
	tar -czf gfmrun_windows_$(VERSION_VALUE).tar.gz gfmrun_windows_$(VERSION_VALUE)

.PHONY: deps
deps:
	go get ./...

.PHONY: clean
clean:
	$(PYTHON) ./plz clean
