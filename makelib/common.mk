# Copyright 2019 Pressinfra SRL. All rights reserved.
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

ifndef __COMMON_MAKEFILE__
__COMMON_MAKEFILE__ := included

# include the common make file
COMMON_SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(COMMON_SELF_DIR)/utils.mk

# remove default suffixes as we dont use them
.SUFFIXES:

# set the shell to bash always
SHELL := /bin/bash

# ====================================================================================
# Host information
# This is defined earlier so that it can be used down the road

# Set the host's OS. Only linux and darwin supported for now
HOSTOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ifeq ($(filter darwin linux,$(HOSTOS)),)
$(error build only supported on linux and darwin host currently)
endif

# Set the host's arch. Only amd64 support for now
HOSTARCH := $(shell uname -m)
ifeq ($(HOSTARCH),x86_64)
HOSTARCH := amd64
endif
ifneq ($(HOSTARCH),amd64)
	$(error build only supported on amd64 host currently)
endif
HOST_PLATFORM := $(HOSTOS)_$(HOSTARCH)

# default target is build
.PHONY: all
all: build

# ====================================================================================
# Helpers

ifeq ($(HOSTOS),darwin)
SED?=sed -i '' -E
else
SED?=sed -i -r
endif

# ====================================================================================
# Build Options

# Set V=1 to turn on more verbose build
V ?= 0
ifeq ($(V),1)
MAKEFLAGS += VERBOSE=1
else
MAKEFLAGS += --no-print-directory
endif

ifeq ($(V),$(filter $(V),1 2 3 4)) # Print verbose make info, iv V>0
ifneq ($(PLATFORM),)
VERBOSE_BUILD_INFO += PLATFORM=$(BLUE)$(PLATFORM)$(CNone)
endif

$(info ---- $(BLUE_BOLD)$(MAKECMDGOALS)$(CNone) $(VERBOSE_BUILD_INFO) )
endif

# Set DEBUG=1 to turn on a debug build
DEBUG ?= 0

# ====================================================================================
# Platform and cross build options

# all supported platforms we build for this can be set to other platforms if desired
# we use the golang os and arch names for convenience
PLATFORMS ?= darwin_amd64 windows_amd64 linux_amd64 linux_arm64

# Set the platform to build if not currently defined
ifeq ($(origin PLATFORM),undefined)

PLATFORM := $(HOST_PLATFORM)

# if the host platform is on the supported list add it to the single build target
ifneq ($(filter $(PLATFORMS),$(HOST_PLATFORM)),)
BUILD_PLATFORMS = $(HOST_PLATFORM)
endif

# for convenience always build the linux platform when building on mac
ifneq ($(HOSTOS),linux)
BUILD_PLATFORMS += linux_amd64
endif

else
BUILD_PLATFORMS = $(PLATFORM)
endif

OS := $(word 1, $(subst _, ,$(PLATFORM)))
ARCH := $(word 2, $(subst _, ,$(PLATFORM)))

ifeq ($(HOSTOS),darwin)
NPROCS := $(shell sysctl -n hw.ncpu)
else
NPROCS := $(shell nproc)
endif

# ====================================================================================
# Setup directories and paths

# include the common make file
COMMON_SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

MAKELIB_BIN_DIR := $(abspath $(shell cd $(COMMON_SELF_DIR)/../bin && pwd -P))

# the root directory of this repo
ifeq ($(origin ROOT_DIR),undefined)
ROOT_DIR := $(abspath $(shell cd $(COMMON_SELF_DIR)/../.. && pwd -P))
endif

# the output directory which holds final build produced artifacts
ifeq ($(origin OUTPUT_DIR),undefined)
OUTPUT_DIR := $(ROOT_DIR)/_output
endif
$(OUTPUT_DIR):
	@mkdir -p "$@"

# the output directory for staged files. The staged files are generated files which are commited to the main repo and
# staged for being pushed to an upstream repo. (eg. generated protobuf client code)
ifeq ($(origin STAGING_DIR),undefined)
STAGING_DIR:= $(ROOT_DIR)/staging/src
endif
$(STAGING_DIR):
	@mkdir -p "$@"

# a working directory that holds all temporary or working items generated
# during the build. The items will be discarded on a clean build and they
# will never be cached.
ifeq ($(origin WORK_DIR), undefined)
WORK_DIR := $(ROOT_DIR)/.work
endif
$(WORK_DIR):
	@mkdir -p "$@"

# a directory that holds tools and other items that are safe to cache
# across build invocations. removing this directory will trigger a
# re-download and waste time. Its safe to cache this directory on CI systems
ifeq ($(origin CACHE_DIR), undefined)
CACHE_DIR := $(ROOT_DIR)/.cache
endif
$(CACHE_DIR):
	@mkdir -p "$@"

TOOLS_DIR := $(CACHE_DIR)/tools
TOOLS_HOST_DIR := $(TOOLS_DIR)/$(HOST_PLATFORM)
TOOLS_BIN_DIR := $(ROOT_DIR)/bin
PATH := $(TOOLS_BIN_DIR):$(PATH)
export PATH

$(TOOLS_HOST_DIR):
	@mkdir -p "$@"

$(TOOLS_BIN_DIR):
	@mkdir -p "$@"

ifeq ($(origin HOSTNAME), undefined)
HOSTNAME := $(shell hostname)
endif

YQ_VERSION ?= 4.11.2
YQ_DOWNLOAD_URL ?= https://github.com/mikefarah/yq/releases/download/v$(YQ_VERSION)/yq_$(HOST_PLATFORM)
$(eval $(call tool.download,yq,$(YQ_VERSION),$(YQ_DOWNLOAD_URL)))

GIT_SEMVER_VERSION ?= 6.1.1
GIT_SEMVER_DOWNLOAD_URL ?= https://github.com/mdomke/git-semver/releases/download/v$(GIT_SEMVER_VERSION)/git-semver_$(GIT_SEMVER_VERSION)_$(HOST_PLATFORM).tar.gz
$(eval $(call tool.download.tar.gz,git-semver,$(GIT_SEMVER_VERSION),$(GIT_SEMVER_DOWNLOAD_URL),git-semver,0))

$(TOOLS_DIR)/git-semver.mk: $(GIT_SEMVER)
	@echo '# dummy target to require installing git-semver before making everything' > "$@"
	@touch "$@"

include $(TOOLS_DIR)/git-semver.mk

# ====================================================================================
# git introspection

ifeq ($(COMMIT_HASH),)
override COMMIT_HASH := $(shell git rev-parse HEAD)
endif

TAGS := $(shell git tag -l --points-at HEAD)

GIT_HEAD := $(shell git rev-parse --abbrev-ref HEAD)

# Set default GIT_TREE_STATE
ifeq ($(shell git status -s | head -c1 | wc -c | tr -d '[[:space:]]'), 0)
GIT_TREE_STATE = clean
else
GIT_TREE_STATE = dirty
endif

# ====================================================================================
# Release Options

CHANNEL ?= master
ifeq ($(filter master alpha beta stable,$(CHANNEL)),)
$(error invalid channel $(CHANNEL))
endif

REMOTE_NAME ?= origin
REMOTE_URL ?= $(shell git remote get-url $(REMOTE_NAME))

# ====================================================================================
# Version and Tagging
#

BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# set a semantic version number from git if VERSION is undefined.
ifeq ($(origin VERSION), undefined)
ifeq ($(GIT_TREE_STATE),clean)
VERSION := $(shell $(GIT_SEMVER) -prefix v $(ROOT_DIR))
else
VERSION := $(shell $(GIT_SEMVER) -prefix v -set-meta $(shell echo "$(COMMIT_HASH)" | head -c8)-dirty $(ROOT_DIR))
endif
else
endif
export VERSION

VERSION_REGEX := ^v([0-9]*)[.]([0-9]*)[.]([0-9]*)(-(alpha|beta|rc)[.][0-9]+)?(\+[[:alnum:].+_-]+)?$$
VERSION_VALID := $(shell echo "$(VERSION)" | grep -E -q '$(VERSION_REGEX)' && echo 1 || echo 0)
VERSION_MAJOR := $(shell echo "$(VERSION)" | sed -E -e 's/$(VERSION_REGEX)/\1/')
VERSION_MINOR := $(shell echo "$(VERSION)" | sed -E -e 's/$(VERSION_REGEX)/\2/')
VERSION_PATCH := $(shell echo "$(VERSION)" | sed -E -e 's/$(VERSION_REGEX)/\3/')

ifeq ($(origin BRANCH_NAME), undefined)
ifeq ($(GIT_HEAD),HEAD)
# pretend we are on a release branch if we are just checking out a commit
BRANCH_NAME := release-$(VERSION_MAJOR).$(VERSION_MINOR)
else
BRANCH_NAME := $(GIT_HEAD)
endif
endif

.publish.tag: .version.require.clean.tree
ifneq ($(VERSION_VALID),1)
	$(error invalid version $(VERSION). must be a semantic semver version (eg. with v[Major].[Minor].[Patch]))
endif
	@$(INFO) tagging commit hash $(COMMIT_HASH) with $(VERSION)
	git tag -f -m "release $(VERSION)" $(VERSION) $(COMMIT_HASH)
	git push $(REMOTE_NAME) $(VERSION)
	@set -e; if ! git ls-remote --heads $(REMOTE_NAME) | grep -q refs/heads/release-$(VERSION_MAJOR).$(VERSION_MINOR); then \
		echo === creating new release branch release-$(VERSION_MAJOR).$(VERSION_MINOR) ;\
		git branch -f release-$(VERSION_MAJOR).$(VERSION_MINOR) $(COMMIT_HASH) ;\
		git push $(REMOTE_NAME) release-$(VERSION_MAJOR).$(VERSION_MINOR) ;\
	fi
	@$(OK) tagging

# fail publish if the version is dirty
.version.require.clean.tree:
	@if [[ $$ALLOW_DIRTY_TREE != "true" ]] && [[ $(GIT_TREE_STATE) = dirty ]]; then \
		$(ERR) version '$(VERSION)' is dirty. The following files changed: ;\
		git status --short;\
		exit 1; \
	fi

.PHONY: .publish.tag .version.require.clean.tree

# ====================================================================================
# Common Targets - Build and Test workflow

# A common target registers a target and calls in order the following targets
# .TARGET.init .TARGET.run .TARGET.done
define common.target
.$(1).init: ; @:
.$(1).run: ; @:
.$(1).done: ; @:

$(1):
	@$$(MAKE) .$(1).init
	@$$(MAKE) .$(1).run
	@$$(MAKE) .$(1).done

.PHONY: $(1) .$(1).init .$(1).run .$(1).done
endef

# lint the code
$(eval $(call common.target,lint))

# run tests
$(eval $(call common.target,test))

# run e2e tests
$(eval $(call common.target,e2e))

# run code generation (eg. compile protocol buffers, collect translations)
$(eval $(call common.target,generate))

# run code auto-formatting tools
$(eval $(call common.target,fmt))

# ====================================================================================
# Release Targets

# publish artifacts
$(eval $(call common.target,publish))
publish: .version.require.clean.tree

# promote all artifacts to a release channel
$(eval $(call common.target,promote))

# tag a release
tag: .publish.tag

.PHONY: tag

# ====================================================================================
# Build targets
#
# These targets are use to build the project artifacts.

# run init steps before building code
# these will run once regardless of how many platforms we are building
.build.init: ; @:

# check the code with fmt, lint, vet and other source level checks pre build
# these will run once regardless of how many platforms we are building
.build.check: ; @:

# check the code with fmt, lint, vet and other source level checks pre build
# these will run for each platform being built
.build.check.platform: ; @:

# build code. this will run once regardless of platform
.build.code: ; @:

# build code. this will run for each platform built
.build.code.platform: ; @:

# build releasable artifacts. this will run once regardless of platform
.build.artifacts: ; @:

# build releasable artifacts. this will run for each platform being built
.build.artifacts.platform: ; @:

# runs at the end of the build to do any cleanup, caching etc.
# these will run once regardless of how many platforms we are building
.build.done: ; @:

# helper targets for building multiple platforms
.do.build.platform.%:
	@$(MAKE) .build.check.platform PLATFORM=$*
	@$(MAKE) .build.code.platform PLATFORM=$*
.do.build.platform: $(foreach p,$(PLATFORMS), .do.build.platform.$(p))

# helper targets for building multiple platforms
.do.build.artifacts.%:
	@$(MAKE) .build.artifacts.platform PLATFORM=$*
.do.build.artifacts: $(foreach p,$(PLATFORMS), .do.build.artifacts.$(p))

# build for all platforms
build.all:
	@$(MAKE) .build.init
	@$(MAKE) .build.check
	@$(MAKE) .build.code
	@$(MAKE) .do.build.platform
	@$(MAKE) .build.artifacts
	@$(MAKE) .do.build.artifacts
	@$(MAKE) .build.done

# build for a single platform if it's supported
build:
ifneq ($(BUILD_PLATFORMS),)
	@$(MAKE) build.all PLATFORMS="$(BUILD_PLATFORMS)"
else
	@:
endif

# Install required build tools. This can be used to generate a docker container with all the required tools for example.
build.tools: |$(TOOLS_HOST_DIR)

# clean all files created during the build.
clean:
	@rm -fr $(OUTPUT_DIR) $(WORK_DIR)

# clean all files created during the build, including caches across builds
distclean: clean
	@rm -fr $(CACHE_DIR)

.PHONY: .build.init .build.check .build.check.platform .build.code .build.code.platform .build.artifacts .build.artifacts.platform
.PHONY: .build.done .do.build.platform.% .do.build.platform .do.build.artifacts.% .do.build.artifacts
.PHONY: build.tools build.info build.all build clean distclean

build.info:
	@echo "build version: $(VERSION)"
	@echo "git commit: $(COMMIT_HASH)"
	@echo "git branch: $(BRANCH_NAME)"
	@echo "git tree state: $(GIT_TREE_STATE)"
	@echo "git tags: $(TAGS)"

# ====================================================================================
# Help

define HELPTEXT
Usage: make [make-options] <target> [options]

Common Targets:
    build              Build source code and other artifacts for host platform.
    build.all          Build source code and other artifacts for all platforms.
    build.tools        Install the required build tools.
    build.vars         Show build vars.
    clean              Remove all files created during the build.
    distclean          Remove all files created during the build including cached tools.
    generate           Run code generation tools.
    fmt                Run code auto-formatting tools.
    lint               Run lint and code analysis tools.
    test               Runs unit tests.
    e2e                Runs end-to-end integration tests.
    translate          Collect translation strings and post-process the .pot/.po files.
    help               Show this help info.

Common Options:
    DEBUG              Whether to generate debug symbols. Default is 0.
    PLATFORM           The platform to build.
    SUITE              The test suite to run.
    TESTFILTER         Tests to run in a suite.
    V                  Build verbosity level (1-4). Default is 0.

Release Targets:
    publish            Build and publish final releasable artifacts
    promote            Promote a release to a release channel
    tag                Tag a release

Release Options:
    VERSION            The version information for binaries and releases.
    CHANNEL            Sets the release channel. Can be set to master, alpha, beta, or stable.

endef
export HELPTEXT

.help: ; @:

help:
	@echo "$$HELPTEXT"
	@$(MAKE) .help

.PHONY: help .help

endif # __COMMON_MAKEFILE__
