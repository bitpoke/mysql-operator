# Copyright 2016 The Upbound Authors. All rights reserved.
# Copyright 2019 The Pressinfra Authors. All rights reserved.
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

ifndef __NODEJS_MAKEFILE__
__NODEJS_MAKEFILE__ := included

# ====================================================================================
# Options

# supported node versions
NODE_SUPPORTED_VERSIONS ?= 10|12
NODE := node

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

# The location of node application within this git repo.
NODE_ROOT_DIR ?= $(SELF_DIR)/../..

# The location of node application source code, relative to the NODE_ROOT_DIR
NODE_SRC_DIR ?= src

NODE_ENV ?= production
export NODE_ENV

YARN := yarn
YARN_MODULE_DIR := $(NODE_ROOT_DIR)/node_modules
YARN_BIN_DIR := $(abspath $(NODE_ROOT_DIR)/node_modules/.bin)
YARN_PACKAGE_FILE := $(NODE_ROOT_DIR)/package.json
YARN_PACKAGE_LOCK_FILE := $(NODE_ROOT_DIR)/yarn.lock

NODE_SRCS ?= $(abspath $(YARN_PACKAGE_FILE)) $(abspath $(YARN_PACKAGE_LOCK_FILE)) $(shell find $(abspath $(NODE_ROOT_DIR)/$(NODE_SRC_DIR)) -type f | grep -v '__tests__')

YARN_CACHE_FOLDER ?= $(CACHE_DIR)/yarn
export YARN_CACHE_FOLDER

YARN_OUTDIR ?= $(OUTPUT_DIR)/yarn
export YARN_OUTDIR

EXTEND_ESLINT ?= true
export EXTEND_ESLINT

ESLINT_OUTPUT_DIR := $(OUTPUT_DIR)/lint/eslint

# ====================================================================================
# NodeJS Tools Targets

ESLINT := $(YARN_BIN_DIR)/eslint
$(ESLINT): |yarn.install
build.tools: $(ESLINT)

# ====================================================================================
# YARN Targets

.PHONY: .yarn.init
.yarn.init:
	@if ! `$(NODE) --version | grep -q -E '^v($(NODE_SUPPORTED_VERSIONS))\.'`; then \
		$(ERR) unsupported node version. Please install one of the following supported version: '$(NODE_SUPPORTED_VERSIONS)' ;\
		exit 1 ;\
	fi

# some node packages like node-sass require platform/arch specific install. we need
# to run yarn for each platform. As a result we track a stamp file per host
YARN_INSTALL_STAMP := $(YARN_MODULE_DIR)/.yarn.install.$(HOST_PLATFORM).stamp

# only run "yarn" if the package.json has changed
$(YARN_INSTALL_STAMP): $(YARN_PACKAGE_FILE) $(YARN_PACKAGE_LOCK_FILE)
	@echo ${TIME} $(BLUE)[TOOL]$(CNone) yarn install
	@cd $(NODE_ROOT_DIR); $(YARN) --silent --frozen-lockfile --non-interactive --production=false || $(FAIL)
	@touch $(YARN_INSTALL_STAMP)
	@$(OK) yarn install

yarn.install: .yarn.init $(YARN_INSTALL_STAMP)

.yarn.clean:
	@rm -rf $(YARN_MODULE_DIR)

.PHONY: yarn.install .yarn.clean

# ====================================================================================
# NodeJS Targets

$(ESLINT_OUTPUT_DIR)/stylecheck.xml: $(ESLINT) $(NODE_SRCS)
	@$(INFO) eslint
	@mkdir -p $(ESLINT_OUTPUT_DIR)
	@cd $(NODE_ROOT_DIR); $(ESLINT) '$(NODE_SRC_DIR)/**/*.{ts,tsx}' --color
	@touch $@
	@$(OK) eslint

js.lint: $(ESLINT_OUTPUT_DIR)/stylecheck.xml

js.lint.fix:
	@$(INFO) eslint fix
	@cd $(NODE_ROOT_DIR); $(ESLINT) '$(NODE_SRC_DIR)/**/*.{ts,tsx}' --color
	@$(OK) eslint fix

# common target for building a node js project
$(eval $(call common.target,js.build))

# common target for testing a node js project
$(eval $(call common.target,js.test))

.PHONY: js.lint js.lint.fix

# ====================================================================================
# Common Targets

.build.init: .yarn.init .js.build.init
.build.check: js.lint
.build.code: .js.build.run
.build.done: .js.build.done

.test.init: .js.test.init
.test.run: .js.test.run
.test.done: .js.test.done

clean: .yarn.clean

.lint.run: js.lint
.fmt.run: js.lint.fix

# ====================================================================================
# Special Targets

define NODEJS_HELPTEXT
nodejs Targets:
    yarn.install       Installs dependencies in a make friendly manner.

endef
export NODEJS_HELPTEXT

.PHONY: .js.help
.js.help:
	@echo "$$NODEJS_HELPTEXT"

.help: .js.help

endif # __NODEJS_MAKEFILE__
