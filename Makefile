PACKAGE := github.com/urfave/gfmxr
ALL_PACKAGES := $(PACKAGE) $(PACKAGE)/cmd/...

VERSION_VAR := $(PACKAGE).VersionString
VERSION_VALUE ?= $(shell git describe --always --dirty --tags 2>/dev/null)
REV_VAR := $(PACKAGE).RevisionString
REV_VALUE ?= $(shell git rev-parse HEAD 2>/dev/null || echo "'???'")
GENERATED_VAR := $(PACKAGE).GeneratedString
GENERATED_VALUE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%S%z')
COPYRIGHT_VAR := $(PACKAGE).CopyrightString
COPYRIGHT_VALUE ?= $(shell grep -i ^copyright LICENSE | sed 's/^[Cc]opyright //')

FIND ?= find
GO ?= go
GOMETALINTER ?= gometalinter
GREP ?= grep
SED ?= sed
TR ?= tr
XARGS ?= xargs

GOPATH := $(shell echo $${GOPATH%%:*})
GOBUILD_LDFLAGS ?= \
	-X '$(VERSION_VAR)=$(VERSION_VALUE)' \
	-X '$(REV_VAR)=$(REV_VALUE)' \
	-X '$(GENERATED_VAR)=$(GENERATED_VALUE)' \
	-X '$(COPYRIGHT_VAR)=$(COPYRIGHT_VALUE)'

.PHONY: all
all: clean test

.PHONY: test
test: deps lint build coverage.html selftest

.PHONY: test-no-cover
test-no-cover:
	$(GO) test -v -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)

.PHONY: test-race
test-race: deps
	$(GO) test -v -race -x -ldflags "$(GOBUILD_LDFLAGS)" $(ALL_PACKAGES)

.PHONY: selftest
selftest:
	FROBS=$$($(GO) run ./cmd/gfmxr/main.go list-frobs | $(TR) "\n" '|' | $(SED) 's/|$$//') ; \
	SELFTEST_COUNT=$$($(GREP) -cE '^``` ('$${FROBS}')' README.md) ; \
	$(GO) run ./cmd/gfmxr/main.go -D -c $${SELFTEST_COUNT}

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
	$(RM) coverage.html coverage.coverprofile $(GOPATH)/bin/gfmxr
	$(FIND) $(GOPATH)/pkg -wholename "*$(PACKAGE)*.a" | $(XARGS) $(RM) -v
