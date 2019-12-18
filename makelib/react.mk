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

ifndef __REACT_MAKEFILE__
__REACT_MAKEFILE__ := included

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(SELF_DIR)/nodejs.mk

# ====================================================================================
# Options

REACT_OUTPUT_DIR ?= $(OUTPUT_DIR)/react

# ====================================================================================
# React app Targets

REACT := $(YARN_BIN_DIR)/react-scripts
$(REACT): yarn.install
build.tools: $(REACT)

$(REACT_OUTPUT_DIR)/index.html: $(REACT) $(NODE_SRCS)
	@$(INFO) react-scripts build
	@cd $(NODE_ROOT_DIR); $(REACT) build
	@mkdir -p $(REACT_OUTPUT_DIR)
	@rm -rf $(REACT_OUTPUT_DIR)
	@mv $(NODE_ROOT_DIR)/build $(REACT_OUTPUT_DIR)
	@$(OK) react-scripts build
react.build: $(REACT_OUTPUT_DIR)/index.html

react.test: $(REACT)
	@$(INFO) react-scripts test
	@cd $(NODE_ROOT_DIR); TZ='UTC' $(REACT) test --env=jsdom --verbose --colors
	@$(OK) react-scripts test

react.run:
	@cd $(NODE_ROOT_DIR); NODE_ENV=development BROWSER=none $(REACT) start

.react.clean:
	@rm -rf $(REACT_OUTPUT_DIR)

.PHONY: react.build react.test .react.clean

# ====================================================================================
# Common Targets

.js.build.run: react.build
clean: .react.clean

# ====================================================================================
# Special Targets

define REACT_HELPTEXT
React Targets:
    react.run          Run the react application for development.

endef
export REACT_HELPTEXT

.PHONY: .react.help
.react.help:
	@echo "$$REACT_HELPTEXT"

.help: .react.help

endif # __REACT_MAKEFILE__
