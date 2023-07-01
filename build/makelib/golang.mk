# Copyright 2016 The Upbound Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ifndef __GOLANG_MAKEFILE__
__GOLANG_MAKEFILE__ := included

# ====================================================================================
# Options

# The go project including repo name, for example, github.com/rook/rook
GO_PROJECT ?= $(PROJECT_REPO)

# Optional. These are subdirs that we look for all go files to test, vet, and fmt
GO_SUBDIRS ?= cmd pkg

# Optional. Additional subdirs used for integration or e2e testings
GO_INTEGRATION_TESTS_SUBDIRS ?=

# Optional directories (relative to CURDIR)
GO_VENDOR_DIR ?= vendor
GO_PKG_DIR ?= $(WORK_DIR)/pkg
GO_CACHE_DIR ?= $(CACHE_DIR)/go

# Optional build flags passed to go tools
GO_BUILDFLAGS ?=
GO_LDFLAGS ?=
GO_TAGS ?=
GO_TEST_TOOL ?= ginkgo
GO_TEST_FLAGS ?=
GO_TEST_SUITE ?=
GO_NOCOV ?=

GO_SRCS := $(shell find $(GO_SUBDIRS) -type f -name '*.go' | grep -v '_test.go')

# ====================================================================================
# Setup go environment

# turn on more verbose build when V=2
ifeq ($(V),2)
GO_LDFLAGS += -v -n
GO_BUILDFLAGS += -x
endif

# whether to generate debug information in binaries. this includes DWARF and symbol tables.
ifeq ($(DEBUG),0)
GO_LDFLAGS += -s -w
endif

# supported go versions
GO_SUPPORTED_VERSIONS ?= 1.16|1.17

# set GOOS and GOARCH
GOOS := $(OS)
GOARCH := $(ARCH)
GOCACHE := $(GO_CACHE_DIR)
export GOOS GOARCH GOCACHE

# set GOOS and GOARCH
GOHOSTOS := $(HOSTOS)
GOHOSTARCH := $(HOSTARCH)

GO_PACKAGES := $(foreach t,$(GO_SUBDIRS),$(GO_PROJECT)/$(t)/...)
GO_TEST_PACKAGES ?= $(shell go list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(foreach t,$(GO_SUBDIRS),$(GO_PROJECT)/$(t)/...))
GO_INTEGRATION_TEST_PACKAGES ?= $(foreach t,$(GO_INTEGRATION_TESTS_SUBDIRS),$(GO_PROJECT)/$(t)/integration)

ifneq ($(GO_TEST_PARALLEL),)
GO_TEST_FLAGS += -p $(GO_TEST_PARALLEL)
endif

ifneq ($(GO_TEST_SUITE),)
ifeq ($(GO_TEST_TOOL),ginkgo)
GO_TEST_FLAGS += -focus '$(GO_TEST_SUITE)'
else # GO_TEST_TOOL != ginkgo
GO_TEST_FLAGS += -run '$(GO_TEST_SUITE)'
endif # GO_TEST_TOOL
endif # GO_TEST_SUITE

GOPATH := $(shell go env GOPATH)

GO := go
GOHOST := GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) $(GO)
GO_VERSION := $(shell $(GO) version | sed -ne 's/[^0-9]*\(\([0-9]\.\)\{0,4\}[0-9][^.]\).*/\1/p')

GO_BIN_DIR := $(abspath $(OUTPUT_DIR)/bin)
GO_OUT_DIR := $(GO_BIN_DIR)/$(PLATFORM)
GO_TEST_DIR := $(abspath $(OUTPUT_DIR)/tests)
GO_TEST_OUTPUT := $(GO_TEST_DIR)/$(PLATFORM)
GO_LINT_DIR := $(abspath $(OUTPUT_DIR)/lint)
GO_LINT_OUTPUT := $(GO_LINT_DIR)/$(PLATFORM)

ifeq ($(GOOS),windows)
GO_OUT_EXT := .exe
endif

ifeq ($(GO_TEST_TOOL),ginkgo)
GO_TEST_FLAGS += -randomizeAllSpecs -randomizeSuites
endif

# NOTE: the install suffixes are matched with the build container to speed up the
# the build. Please keep them in sync.

# we run go build with -i which on most system's would want to install packages
# into the system's root dir. using our own pkg dir avoid thats
ifneq ($(GO_PKG_DIR),)
GO_PKG_BASE_DIR := $(abspath $(GO_PKG_DIR)/$(PLATFORM))
GO_PKG_STATIC_FLAGS := -pkgdir $(GO_PKG_BASE_DIR)_static
endif

GO_COMMON_FLAGS = $(GO_BUILDFLAGS) -tags '$(GO_TAGS)'
GO_STATIC_FLAGS = $(GO_COMMON_FLAGS) $(GO_PKG_STATIC_FLAGS) -installsuffix static  -ldflags '$(GO_LDFLAGS)'
GO_GENERATE_FLAGS = $(GO_BUILDFLAGS) -tags 'generate $(GO_TAGS)'
GO_XGETTEXT_ARGS ?=
GO_LOCALE_PREFIX ?= default

export GO111MODULE

# switch for go modules
ifeq ($(GO111MODULE),on)

# set GOPATH to $(GO_PKG_DIR), so that the go modules are installed there, instead of the default $HOME/go/pkg/mod
export GOPATH=$(abspath $(GO_PKG_DIR))

GO_SRCS += go.mod go.sum

else

GO_SRCS += Gopkg.toml Gopkg.lock

endif

# ====================================================================================
# Go Tools macros

# Creates a target for downloading and compiling a go tool from source
# 1 tool, 2 version, 3 tool url, 4 go env vars
define tool.go.get
$(call tool,$(1),$(2))

$$(TOOLS_HOST_DIR)/$(1)-v$(2): |$$(TOOLS_HOST_DIR)
	@echo ${TIME} ${BLUE}[TOOL]${CNone} go get $(3)@$(2)
	@mkdir -p $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) || $$(FAIL)
	@mkdir -p $$(TOOLS_DIR)/go/$(1)-v$(2)/ && cd $$(TOOLS_DIR)/go/$(1)-v$(2)/ && $(GOHOST) mod init tools && \
	    $(4) GO111MODULE=on GOPATH=$$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) $(GOHOST) get -u $(3)@$(2) || $$(FAIL)
	@find $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) -type f -print0 | xargs -0 chmod 0644 || $$(FAIL)
	@find $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) -type d -print0 | xargs -0 chmod 0755 || $$(FAIL)
	@mv $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2)/bin/$(1) $$@ || $$(FAIL)
	@chmod +x $$@
	@rm -rf $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2)
	@$$(OK) go get $(3)
endef # tool.go.get

# Creates a target for installing a go tool from source
# 1 tool, 2 version, 3 tool url, 4 go env vars
define tool.go.install
$(call tool,$(1),$(2))

$$(TOOLS_HOST_DIR)/$(1)-v$(2): |$$(TOOLS_HOST_DIR)
	@echo ${TIME} ${BLUE}[TOOL]${CNone} go install $(3)@$(2)
	@mkdir -p $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) || $$(FAIL)
	@mkdir -p $$(TOOLS_DIR)/go/$(1)-v$(2)/ && cd $$(TOOLS_DIR)/go/$(1)-v$(2)/ && $(GOHOST) mod init tools && \
	    $(4) GO111MODULE=on GOPATH=$$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) $(GOHOST) install $(3)@$(2) || $$(FAIL)
	@find $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) -type f -print0 | xargs -0 chmod 0644 || $$(FAIL)
	@find $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2) -type d -print0 | xargs -0 chmod 0755 || $$(FAIL)
	@mv $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2)/bin/$(1) $$@ || $$(FAIL)
	@chmod +x $$@
	@rm -rf $$(TOOLS_HOST_DIR)/tmp-$(1)-v$(2)
	@$$(OK) go install $(3)
endef # tool.go.install

# Creates a target for compiling a vendored go tool
# 1 tool, 2 package
define tool.go.vendor.install
$(subst -,_,$(call upper,$(1))) := $$(TOOLS_BIN_DIR)/$(1)

build.tools: $$(TOOLS_BIN_DIR)/$(1)
$$(TOOLS_BIN_DIR)/$(1): vendor/$(2)
	@echo ${TIME} ${BLUE}[TOOL]${CNone} installing go vendored $(2)
	@GOBIN=$$(TOOLS_BIN_DIR) $$(GOHOST) install ./vendor/$(2)
	@$$(OK) installing go vendored $(2)

endef # tool.go.vendor.install

# ====================================================================================
# Tools install targets

DEP_VERSION ?= 0.5.4
DEP_DOWNLOAD_URL ?= https://github.com/golang/dep/releases/download/v$(DEP_VERSION)/dep-$(HOSTOS)-$(HOSTARCH)
$(eval $(call tool.download,dep,$(DEP_VERSION),$(DEP_DOWNLOAD_URL)))

GOLANGCI_LINT_VERSION ?= 1.41.1
GOLANGCI_LINT_DOWNLOAD_URL ?= https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(HOSTOS)-$(HOSTARCH).tar.gz
$(eval $(call tool.download.tar.gz,golangci-lint,$(GOLANGCI_LINT_VERSION),$(GOLANGCI_LINT_DOWNLOAD_URL)))

ifneq ($(LANGUAGES),)
GO_XGETTEXT_VERSION ?= v0.0.0-20180127124228-c366ce0fe48d
GO_XGETTEXT_URL ?= github.com/presslabs/gettext/go-xgettext
$(eval $(call tool.go.install,go-xgettext,$(GO_XGETTEXT_VERSION),$(GO_XGETTEXT_URL)))
endif

# we use a consistent version of gofmt even while running different go compilers.
# see https://github.com/golang/go/issues/26397 for more details
GOFMT_VERSION ?= 1.16.6
GOFMT_DOWNLOAD_URL ?= https://dl.google.com/go/go$(GOFMT_VERSION).$(HOSTOS)-$(HOSTARCH).tar.gz
ifneq ($(findstring $(GOFMT_VERSION),$(GO_VERSION)),)
GOFMT := $(shell which gofmt)
else
$(eval $(call tool.download.tar.gz,gofmt,$(GOFMT_VERSION),$(GOFMT_DOWNLOAD_URL),bin/gofmt))
endif

GOIMPORTS_VERSION ?= v0.1.5
GOIMPORTS_URL ?= golang.org/x/tools/cmd/goimports
$(eval $(call tool.go.install,goimports,$(GOIMPORTS_VERSION),$(GOIMPORTS_URL)))

ifeq ($(GO_TEST_TOOL),ginkgo)
GINKGO_VERSION ?= v1.16.4
GINKGO_URL ?= github.com/onsi/ginkgo/ginkgo
$(eval $(call tool.go.install,ginkgo,$(GINKGO_VERSION),$(GINKGO_URL)))
else # GO_TEST_TOOL != ginkgo
GO_JUNIT_REPORT_VERSION ?= v0.9.2-0.20191008195320-984a47ca6b0a
GO_JUNIT_REPORT_URL ?= github.com/jstemmer/go-junit-report
$(eval $(call tool.go.install,go-junit-report,$(GO_JUNIT_REPORT_VERSION),$(GO_JUNIT_REPORT_URL),go-junit-report))
endif # GO_TEST_TOOL

# ====================================================================================
# Go Targets

.go.init:
	@if ! `$(GO) version | grep -q -E '\bgo($(GO_SUPPORTED_VERSIONS))\b'`; then \
		$(ERR) unsupported go version. Please make install one of the following supported version: '$(GO_SUPPORTED_VERSIONS)' ;\
		exit 1 ;\
	fi
	@if [ "$(GO111MODULE)" != "on" ] && [ "$(realpath ../../../..)" !=  "$(realpath $(GOPATH))" ]; then \
		$(WARN) the source directory is not relative to the GOPATH at $(GOPATH) or you are you using symlinks. The build might run into issue. Please move the source directory to be at $(GOPATH)/src/$(GO_PROJECT) ;\
	fi

# common target for building a node js project
$(eval $(call common.target,go.build))
.go.build.run: .do.go.build
.do.go.build:
	@$(INFO) go build $(PLATFORM) $(GO_TAGS)
	$(foreach p,$(GO_STATIC_PACKAGES),@CGO_ENABLED=0 $(GO) build -v -o $(GO_OUT_DIR)/$(call list-join,_,$(lastword $(subst /, ,$(p))) $(call lower,$(GO_TAGS)))$(GO_OUT_EXT) $(GO_STATIC_FLAGS) $(p) || $(FAIL) ${\n})
	@$(OK) go build $(PLATFORM) $(GO_TAGS)

ifeq ($(GO_TEST_TOOL),ginkgo)
go.test.unit: $(GINKGO)
	@$(INFO) ginkgo unit-tests
	@mkdir -p $(GO_TEST_OUTPUT)
	@CGO_ENABLED=0 $(GINKGO) $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(patsubst $(ROOT_DIR)/%,./%,$(shell go list -f '{{ .Dir }}' $(GO_TEST_PACKAGES))) || $(FAIL)
	@$(OK) go test unit-tests

go.test.integration: $(GINKGO)
	@$(INFO) ginkgo integration-tests
	@mkdir -p $(GO_TEST_OUTPUT) || $(FAIL)
	@CGO_ENABLED=0 $(GINKGO) $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(foreach t,$(GO_INTEGRATION_TESTS_SUBDIRS),./$(t)/...) $(TEST_FILTER_PARAM) || $(FAIL)
	@$(OK) go test integration-tests

else # GO_TEST_TOOL != ginkgo
go.test.unit: $(GO_JUNIT_REPORT)
	@$(INFO) go test unit-tests
	@mkdir -p $(GO_TEST_OUTPUT)
	@CGO_ENABLED=0 $(GOHOST) test $(GO_STATIC_FLAGS) $(GO_TEST_PACKAGES) || $(FAIL)
	@CGO_ENABLED=0 $(GOHOST) test $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(GO_TEST_PACKAGES) $(TEST_FILTER_PARAM) 2>&1 | tee $(GO_TEST_OUTPUT)/unit-tests.log || $(FAIL)
	@cat $(GO_TEST_OUTPUT)/unit-tests.log | $(GO_JUNIT_REPORT) -set-exit-code > $(GO_TEST_OUTPUT)/unit-tests.xml || $(FAIL)
	@$(OK) go test unit-tests

go.test.integration: $(GO_JUNIT_REPORT)
	@$(INFO) go test integration-tests
	@mkdir -p $(GO_TEST_OUTPUT) || $(FAIL)
	@CGO_ENABLED=0 $(GOHOST) test -i $(GO_STATIC_FLAGS) $(GO_INTEGRATION_TEST_PACKAGES) || $(FAIL)
	@CGO_ENABLED=0 $(GOHOST) test $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(GO_INTEGRATION_TEST_PACKAGES) $(TEST_FILTER_PARAM) 2>&1 | tee $(GO_TEST_OUTPUT)/integration-tests.log || $(FAIL)
	@cat $(GO_TEST_OUTPUT)/integration-tests.log | $(GO_JUNIT_REPORT) -set-exit-code > $(GO_TEST_OUTPUT)/integration-tests.xml || $(FAIL)
	@$(OK) go test integration-tests

endif # GO_TEST_TOOL

$(GO_LINT_OUTPUT)/stylecheck.xml: $(GO_SRCS) $(GOLANGCI_LINT)
	@$(INFO) golangci-lint
	@mkdir -p $(GO_LINT_OUTPUT)
	@$(GOLANGCI_LINT) run $(GO_LINT_ARGS) || $(FAIL)
	@touch $(GO_LINT_OUTPUT)/stylecheck.xml
	@$(OK) golangci-lint

go.lint: $(GO_LINT_OUTPUT)/stylecheck.xml

go.fmt.verify: $(GOFMT) .go.imports.verify
	@$(INFO) go fmt verify
	@gofmt_out=$$($(GOFMT) -s -d -e $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) 2>&1) && [ -z "$${gofmt_out}" ] || (echo "$${gofmt_out}" 1>&2; $(FAIL))
	@$(OK) go fmt verify

go.fmt: $(GOFMT) .go.imports.fix
	@$(INFO) gofmt simplify
	@$(GOFMT) -l -s -w $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) || $(FAIL)
	@$(OK) gofmt simplify

.go.imports.verify: $(GOIMPORTS)
	@$(INFO) goimports verify
	@goimports_out=$$($(GOIMPORTS) -d -e -local $(GO_PROJECT) $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) 2>&1) && [ -z "$${goimports_out}" ] || (echo "$${goimports_out}" 1>&2; $(FAIL))
	@$(OK) goimports verify

.go.imports.fix: $(GOIMPORTS)
	@$(INFO) goimports fix
	@$(GOIMPORTS) -l -w -local $(GO_PROJECT) $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) || $(FAIL)
	@$(OK) goimports fix

ifeq ($(GO111MODULE),on)

.go.vendor.lite go.vendor.check:
	@$(INFO) verify dependencies have expected content
	@$(GOHOST) mod verify || $(FAIL)
	@$(OK) go modules dependencies verified

go.vendor.update:
	@$(INFO) update go modules
	@$(GOHOST) get -u ./... || $(FAIL)
	@$(OK) update go modules

go.vendor:
	@$(INFO) go mod vendor
	@$(GOHOST) mod vendor || $(FAIL)
	@$(OK) go mod vendor

else

.go.vendor.lite: $(DEP)
#	dep ensure blindly updates the whole vendor tree causing everything to be rebuilt. This workaround
#	will only call dep ensure if the .lock file changes or if the vendor dir is non-existent.
	@if [ ! -d $(GO_VENDOR_DIR) ]; then \
		$(MAKE) go.vendor; \
	elif ! $(DEP) ensure -no-vendor -dry-run &> /dev/null; then \
		$(MAKE) go.vendor; \
	fi

go.vendor.check: $(DEP)
	@$(INFO) checking if vendor deps changed
	@$(DEP) check -skip-vendor || $(FAIL)
	@$(OK) vendor deps have not changed

go.vendor.update: $(DEP)
	@$(INFO) updating vendor deps
	@$(DEP) ensure -update -v || $(FAIL)
	@$(OK) updating vendor deps

go.vendor: $(DEP)
	@$(INFO) dep ensure
	@$(DEP) ensure || $(FAIL)
	@$(OK) dep ensure

endif

.go.clean:
	@# `go modules` creates read-only folders under WORK_DIR
	@# make all folders within WORK_DIR writable, so they can be deleted
	@if [ -d $(WORK_DIR) ]; then chmod -R +w $(WORK_DIR); fi

	@rm -fr $(GO_BIN_DIR) $(GO_TEST_DIR)

.go.distclean:
	@rm -rf $(GO_VENDOR_DIR) $(GO_PKG_DIR)

go.generate: $(GOIMPORTS)
	@$(INFO) go generate $(PLATFORM)
	@CGO_ENABLED=0 $(GOHOST) generate $(GO_GENERATE_FLAGS) $(GO_PACKAGES) $(GO_INTEGRATION_TEST_PACKAGES) || $(FAIL)
	@find $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) -type f -name 'zz_generated*' -exec $(GOIMPORTS) -l -w -local $(GO_PROJECT) {} \;
	@$(OK) go generate $(PLATFORM)

ifneq ($(LANGUAGES),)
.PHONY: go.collect-translations
go.collect-translations: $(GO_XGETTEXT) |$(WORK_DIR)
	@$(INFO) go-xgettext collect translations
	@$(GO_XGETTEXT) -sort-output -output "$(WORK_DIR)/$(GO_LOCALE_PREFIX).pot" $(GO_XGETTEXT_ARGS) \
		$(shell find $(GO_SUBDIRS) $(STAGING_DIR) -type f -name '*.go' -not -name '*test.go' -not -name '*pb.go')
	@$(SED) 's|$(STAGING_DIR)/||g' "$(WORK_DIR)/$(GO_LOCALE_PREFIX).pot" || $(FAIL)
	$(foreach p,$(GO_SUBDIRS),@$(SED) 's|([[:space:]])($(p)/[[:alnum:]/]+.go)|\1$(GO_PROJECT)/\2|g' "$(WORK_DIR)/$(GO_LOCALE_PREFIX).pot" || $(FAIL) ${\n})
#
#	Update the .pot file only if there are changes to actual messages. We need this because the collector always updates
#	the POT-Creation-Date
#
	@$(MAKELIB_BIN_DIR)/po-diff.sh $(LOCALES_DIR)/$(GO_LOCALE_PREFIX).pot $(WORK_DIR)/$(GO_LOCALE_PREFIX).pot || \
		mv $(WORK_DIR)/$(GO_LOCALE_PREFIX).pot $(LOCALES_DIR)/$(GO_LOCALE_PREFIX).pot
	@rm -f $(WORK_DIR)/$(GO_LOCALE_PREFIX).pot
#
	@$(OK) go-xgettext collect translations

.translations.init: go.collect-translations
endif

.PHONY: .go.init .do.go.build go.test.unit go.test.integration go.test.codecov go.lint go.fmt go.generate
.PHONY: .go.vendor.lite go.vendor go.vendor.check go.vendor.update .go.clean .go.distclean

# ====================================================================================
# Common Targets

.build.init: .go.init .go.vendor.lite
.build.check: go.lint
.build.code.platform: go.build
.clean: .go.clean
.distclean: .go.distclean

.lint.init: .go.init
.lint.run: go.fmt.verify go.lint

.test.init: .go.init
.test.run: go.test.unit

.e2e.init: .go.init
.e2e.run: go.test.integration

.fmt.run: go.fmt

.generate.run: go.generate

# ====================================================================================
# Special Targets

define GO_HELPTEXT
Go Targets:
    go.vendor          Updates vendor packages.
    go.vendor.check    Fail the build if vendor packages have changed.
    go.vendor.update   Update vendor dependencies.

Go Options:
    GO_TEST_PACKAGES   Packages to run the tests for.
    GO_TEST_SUITE      Regex filter for the tests.

endef
export GO_HELPTEXT

.PHONY: .go.help
.go.help:
	@echo "$$GO_HELPTEXT"

.help: .go.help

endif # __GOLANG_MAKEFILE__
