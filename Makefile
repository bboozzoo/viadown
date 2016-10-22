GO ?= go
V ?= 0
PKGS = $(shell go list ./... | grep -v vendor)
VERSION = $(shell git describe --tags --always --dirty)

GO_LDFLAGS = \
	-ldflags "-X main.Version=$(VERSION)"

ifeq ($(V),1)
BUILDV = -v
endif

build:
	$(GO) build $(GO_LDFLAGS) $(BUILDV)

install:
	$(GO) install $(GO_LDFLAGS) $(BUILDV)

clean:
	$(GO) clean
	rm -f coverage.out coverage-tmp.out

test:
	$(GO) test -v $(PKGS)

cover: coverage
	$(GO) tool cover -func=coverage.out

htmlcover: coverage
	$(GO) tool cover -html=coverage.out

coverage:
	rm -f coverage.out
	echo 'mode: set' > coverage.out
	for p in $$($(GO) list ./... | grep -v /vendor/); do \
		rm -f coverage-tmp.out;  \
		$(GO) test -coverprofile=coverage-tmp.out $$p ; \
		cat coverage-tmp.out |grep -v 'mode:' >> coverage.out; \
	done

.PHONY: build clean test check cover htmlcover coverage
