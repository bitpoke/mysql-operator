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

ifndef __PROTOBUF_MAKEFILE__
__PROTOBUF_MAKEFILE__ := included

# ====================================================================================
# Setup protobuf environment

PROTOBUF_DIR ?= proto
PROTOBUF_FILES ?= $(sort $(shell find $(PROTOBUF_DIR) -name "*.proto"))

PROTOC_VERSION ?= 3.10.1

# ====================================================================================
# Tools install targets

PROTOTOOL_VERSION ?= 1.9.0
PROTOTOOL_CACHE_PATH := $(TOOLS_HOST_DIR)/prototool
export PROTOTOOL_CACHE_PATH

PROTOTOOL_DOWNLOAD_URL ?= https://github.com/uber/prototool/releases/download/v$(PROTOTOOL_VERSION)/prototool-$(HOSTOS)-x86_64
$(eval $(call tool.download,prototool,$(PROTOTOOL_VERSION),$(PROTOTOOL_DOWNLOAD_URL)))

# ====================================================================================
# Protobuf Targets

build.tools: .pb.prototool.cache.update
.pb.prototool.cache.update: $(PROTOTOOL_CACHE_PATH)/.update
$(PROTOTOOL_CACHE_PATH)/.update: $(PROTOBUF_DIR)/prototool.yaml |$(PROTOTOOL)
	@echo ${TIME} $(BLUE)[TOOL]$(CNone) updating prototool cache
	@$(PROTOTOOL) cache update $(PROTOBUF_DIR)
	@touch $@
	@$(OK) updating prototool cache

.pb.init: .pb.prototool.cache.update

pb.lint: $(PROTOTOOL)
	@$(INFO) prototool lint
	@$(PROTOTOOL) lint $(PROTOBUF_DIR) || $(FAIL)
	@$(OK) prototool lint

pb.fmt.verify: $(PROTOTOOL)
	@$(INFO) prototool format verify
	@$(PROTOTOOL) format -l $(PROTOBUF_DIR) || $(FAIL)
	@$(OK) prototool format verify

pb.fmt: $(PROTOTOOL)
	@$(INFO) prototool format
	@$(PROTOTOOL) format -w $(PROTOBUF_DIR) || $(FAIL)
	@$(OK) prototool format

# expose as common target so that we can hook in other generators
# eg. https://github.com/dcodeIO/protobuf.js
$(eval $(call common.target,pb.generate))

.pb.prototool.generate:
	@$(INFO) prototool generate
	@$(PROTOTOOL) generate $(PROTOBUF_DIR)
	@$(OK) prototool generate

.pb.generate.init: .pb.init
.pb.generate.run: .pb.prototool.generate

.PHONY: .go.init go.lint go.fmt go.generate .pb.clean .pb.distclean
.PHONY: .pb.prototool.cache.update .pb.prototool.generate

# ====================================================================================
# Common Targets

.lint.init: .pb.init
.lint.run: pb.fmt.verify pb.lint

.fmt.run: pb.fmt

.generate.init: .pb.init
.generate.run: pb.generate

# ====================================================================================
# Special Targets

define PROTOBUF_HELPTEXT
Protobuf Targets:
    pb.generate        Generate code from protobuf files in $(PROTOBUF_DIR)

endef
export PROTOBUF_HELPTEXT

.PHONY: .go.help
.pb.help:
	@echo "$$PROTOBUF_HELPTEXT"

.help: .pb.help


# # we use a consistent version of gofmt even while running different go compilers.
# # see https://github.com/golang/go/issues/26397 for more details
# PROTOC_VERSION ?= 3.10.1
# PROTOC_DOWNLOAD_URL ?= https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(HOSTOS)-$(HOSTARCH).zip
# $(eval $(call tool.download.zip,protoc,$(PROTOC_VERSION),$(PROTOC_DOWNLOAD_URL),bin/protoc))

endif # __PROTOBUF_MAKEFILE__
