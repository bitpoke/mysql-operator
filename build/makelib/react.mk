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

REACT_LOCALE_PREFIX ?= messages

# ====================================================================================
# React app Targets

REACT := $(YARN_BIN_DIR)/react-scripts --max_old_space_size=4096
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

react.run: $(REACT)
	@cd $(NODE_ROOT_DIR); NODE_ENV=development BROWSER=none $(REACT) start

.react.clean:
	@rm -rf $(REACT_OUTPUT_DIR)

.PHONY: react.build react.test .react.clean

ifneq ($(LANGUAGES),)
I18NEXT_CONV := $(YARN_BIN_DIR)/i18next-conv
REACT_GETTEXT_PARSER := $(YARN_BIN_DIR)/react-gettext-parser

$(I18NEXT_CONV): yarn.install
$(REACT_GETTEXT_PARSER): yarn.install
build.tools: $(REACT_GETTEXT_PARSER) $(I18NEXT_CONV)

.PHONY: react.collect-translations
react.collect-translations: $(REACT_GETTEXT_PARSER) |$(WORK_DIR)
	@$(INFO) react-gettext-parser collect translations
	@cd $(NODE_ROOT_DIR); $(REACT_GETTEXT_PARSER) --config .gettextparser --no-wrap --output $(abspath $(WORK_DIR))/$(REACT_LOCALE_PREFIX).pot '$(NODE_SRC_DIR)/**/*.{js,ts,tsx}'
#	Update the .pot file only if there are changes to actual messages. We need this because the collector always updates
#	the POT-Creation-Date
#
	@$(MAKELIB_BIN_DIR)/po-diff.sh $(LOCALES_DIR)/$(REACT_LOCALE_PREFIX).pot $(WORK_DIR)/$(REACT_LOCALE_PREFIX).pot || \
		mv $(WORK_DIR)/$(REACT_LOCALE_PREFIX).pot $(LOCALES_DIR)/$(REACT_LOCALE_PREFIX).pot
	@rm -f $(WORK_DIR)/$(REACT_LOCALE_PREFIX).pot
#
	@$(OK) react-gettext-parser collect translations

react.convert-translations: $(I18NEXT_CONV)
	@$(INFO) i18next convert translations to json
	$(foreach l,$(LANGUAGES),@$(I18NEXT_CONV) --language $(l) --skipUntranslated \
		--source $(LOCALES_DIR)/$(l)/$(REACT_LOCALE_PREFIX).po \
		--target $(NODE_ROOT_DIR)/$(NODE_SRC_DIR)/locales/$(l).json  \
		> /dev/null || $(FAIL) ${\n}\
	)
	@$(OK) i18next convert translations to json

.translations.init: react.collect-translations
.translations.done: react.convert-translations
endif

# ====================================================================================
# Common Targets

.js.build.run: react.build
.js.test.run: react.test
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
