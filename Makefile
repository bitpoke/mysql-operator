DATE    = $(shell date +%Y%m%d%H%M)
IMAGE   ?= presslabs/titanium
VERSION = v$(DATE)
GOOS    ?= $(shell go env | grep GOOS | cut -d'"' -f2)
BINARY  := operator

LDFLAGS := -X github.com/presslabs/titanium/pkg/operator.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

SRCDIRS  := cmd pkg
PACKAGES := $(shell find $(SRCDIRS) -type d)
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))

# A list of all types.go files in pkg/apis
TYPES_FILES := $(shell find pkg/apis -name types.go)
HACK_DIR ?= hack
GOPATH ?= $(HOME)/go

.PHONY: all clean generate

all: bin/$(GOOS)/$(BINARY)

generate: $(TYPES_FILES)
	GOPATH=$(GOPATH) $(HACK_DIR)/update-codegen.sh

bin/%/$(BINARY): $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build $(GOFLAGS) -v -i -o bin/$*/$(BINARY) $<

build: bin/linux/$(BINARY)
	docker build -t $(IMAGE):$(VERSION) .

push:
	docker push $(IMAGE):$(VERSION)

clean:
	rm -rf bin/*
