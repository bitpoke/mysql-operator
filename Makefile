PACKAGE_NAME := github.com/presslabs/mysql-operator
REGISTRY := quay.io/presslabs
IMAGE_TAGS := canary
BUILD_TAG := build

SRCDIRS  := cmd pkg
PACKAGES := $(shell go list ./... | grep -v /vendor)
GOFILES  := $(shell find $(SRCDIRS) -name '*.go' -type f | grep -v '_test.go')

ifeq ($(APP_VERSION),)
APP_VERSION := $(shell git describe --abbrev=7 --dirty --tags --always)
endif

GIT_COMMIT ?= $(shell git rev-parse HEAD)

ifeq ($(shell git status --porcelain),)
	GIT_STATE ?= clean
else
	GIT_STATE ?= dirty
endif

HACK_DIR ?= hack
GOPATH ?= $(HOME)/go
GOBIN :=${GOPATH}/bin
GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH ?= amd64
GOFLAGS := -ldflags "-X $(PACKAGE_NAME)/pkg/util.AppGitState=${GIT_STATE} -X $(PACKAGE_NAME)/pkg/util.AppGitCommit=${GIT_COMMIT} -X $(PACKAGE_NAME)/pkg/util.AppVersion=${APP_VERSION}"

# Get a list of all binaries to be built
CMDS     := $(shell find ./cmd/ -maxdepth 1 -type d -exec basename {} \; | grep -v cmd)
BIN_CMDS := $(patsubst %, bin/%_$(GOOS)_$(GOARCH), $(CMDS))
DOCKER_BIN_CMDS := $(patsubst %, $(HACK_DIR)/docker/%/%, $(CMDS))

.DEFAULT_GOAL := bin/mysql-operator_$(GOOS)_$(GOARCH)

# Code building targets
#######################
.PHONY: build
build: $(BIN_CMDS)

bin/%: $(GOFILES) Makefile
	CGO_ENABLED=0 \
	GOOS=$(shell echo "$*" | cut -d'_' -f2) \
	GOARCH=$(shell echo "$*" | cut -d'_' -f3) \
		go build $(GOFLAGS) \
			-v -o $@ cmd/$(shell echo "$*" | cut -d'_' -f1)/main.go

# Testing targets
#################
.PHONY: test
test:
	go test -v \
	    -race \
		$$(go list ./... | \
			grep -v '/vendor/' | \
			grep -v '/test/e2e' | \
			grep -v '/pkg/generated/' | \
			grep -v '/pkg/client' \
		)

.PHONY: full-test
full-test: generate_verify test

.PHONY: lint
lint:
	@set -e; \
	GO_FMT=$$(git ls-files *.go | grep -v 'vendor/' | xargs gofmt -d); \
	if [ -n "$${GO_FMT}" ] ; then \
		echo "Please run go fmt"; \
		echo "$$GO_FMT"; \
		exit 1; \
	fi

# Cleanup targets
#################
.PHONY: clean
clean:
	rm -rf bin/*

# Docker image targets
######################
.PHONY: install-docker
install-docker : $(patsubst %, bin/%_linux_amd64, $(CMDS))
	set -e;
		for cmd in $(CMDS); do \
			install -m 755 bin/$${cmd}_linux_amd64 $(HACK_DIR)/docker/$${cmd}/$${cmd} ; \
		done

.PHONY: images
images: install-docker
	set -e;
	for cmd in $(CMDS); do \
		docker build \
			--build-arg VCS_REF=$(GIT_COMMIT) \
			--build-arg APP_VERSION=$(APP_VERSION) \
			-t $(REGISTRY)/$${cmd}:$(BUILD_TAG) \
			-f $(HACK_DIR)/docker/$${cmd}/Dockerfile $(HACK_DIR)/docker/$${cmd} ; \
	done

publish: images
	set -e; \
	for cmd in $(CMDS); do \
		for tag in $(IMAGE_TAGS); do \
			docker tag $(REGISTRY)/$${cmd}:$(BUILD_TAG) $(REGISTRY)/$${cmd}:$${tag}; \
			docker push $(REGISTRY)/$${cmd}:$${tag}; \
		done ; \
	done


# Code generation targets
#########################
CODEGEN_APIS_VERSIONS := mysql:v1alpha1
CODEGEN_TOOLS := deepcopy client lister informer openapi
CODEGEN_APIS_PKG := $(PACKAGE_NAME)/pkg/apis
CODEGEN_OUTPUT_PKG := $(PACKAGE_NAME)/pkg/generated
CODEGEN_OPENAPI_PKG := $(PACKAGE_NAME)/pkg/openapi
include hack/codegen.mk


# CRD generator
###############
CHART_TEMPLATE_PATH := deploy/
CRDS := mysqlcluster mysqlbackup

CRD_GEN_FILES := $(addprefix $(CHART_TEMPLATE_PATH),$(addsuffix .yaml,$(CRDS)))

$(CRD_GEN_FILES):
	go run hack/crds/main.go $(basename $(notdir $@)) \
		--annotations "helm.sh/hook=pre-install" \
		--labels "chart={{ template \"mysql-operator.chart\" . }}" \
		>> $@


.PHONEY: generate-yaml clean-yaml
generate-yaml: clean-yaml $(CRD_GEN_FILES)

clean-yaml:
	rm -f $(CRD_GEN_FILES)
