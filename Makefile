PACKAGE := github.com/urfave/gfmrun
ALL_PACKAGES := $(PACKAGE) $(PACKAGE)/cmd/...

GIT ?= git
GO ?= go
GOMETALINTER ?= gometalinter
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
	$(GOMETALINTER) --errors --tests -D gocyclo --deadline=1m ./...

.PHONY: lintbomb
lintbomb:
	$(GOMETALINTER) --tests -D gocyclo --deadline=1m ./...

.PHONY: build
build: deps
	$(GO) install -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)

.PHONY: deps
deps:
	$(GO) get -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)
	$(GO) get -t -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)

.PHONY: clean
clean:
	$(PYTHON) ./plz clean
