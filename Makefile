DATE    = $(shell date +%Y%m%d%H%M)
IMAGE   ?= presslabs/titanium
VERSION = v$(DATE)
GOOS    ?= $(shell go env | grep GOOS | cut -d'"' -f2)
BINARY  := operator

LDFLAGS :=  -X github.com/presslabs/titanium/pkg/operator.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

SRCDIRS  := cmd pkg
PACKAGES := $(shell find $(SRCDIRS) -type d)
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))

TEST_FILES := $(addsuffix /*_test.go,$(PACKAGES))
TEST_FILES := $(wildcard $(TEST_FILES))

# A list of all types.go files in pkg/apis
TYPES_FILES := $(shell find pkg/apis -name types.go)
HACK_DIR ?= hack
GOPATH ?= $(HOME)/go

.PHONY: all clean generate

all: bin/$(GOOS)/$(BINARY)

generate: $(TYPES_FILES)
	GOPATH=$(GOPATH) $(HACK_DIR)/update-codegen.sh

bin/%/$(BINARY): $(GOFILES) Makefile
	CGO_ENABLED=0 GOOS=$* GOARCH=amd64 go build $(GOFLAGS) \
				-i -installsuffix titan -v -o bin/$*/$(BINARY) $<

test: $(TEST_FILES)
	go test -v -installsuffix titan_test \
	    -race \
			$$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/pkg/generated/' \
		)

clean:
	rm -rf bin/*
