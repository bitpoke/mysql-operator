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

ifndef __GETTEXT_MAKEFILE__
__GETTEXT_MAKEFILE__ := included

# ====================================================================================
# Options

# ====================================================================================
# Translations

# The list of languages to generate translations for
LANGUAGES ?=

LOCALES_DIR ?= $(ROOT_DIR)/locales
$(LOCALES_DIR):
	@mkdir -p $(LOCALES_DIR)

ifeq ($(LANGUAGES),)
$(error You must specify the LANGUAGES variable in order to handle translations)
endif

ifeq ($(HOSTOS),darwin)
MSGFMT = /usr/local/opt/gettext/bin/msgfmt
MSGMERGE = /usr/local/opt/gettext/bin/msgmerge
else
MSGFMT = msgfmt
MSGMERGE = msgmerge
endif

PO_FILES := $(shell find $(LOCALES_DIR) -name '*.po')
POT_FILES := $(shell find $(LOCALES_DIR) -mindepth 1 -maxdepth 1 -name '*.pot')

# lint the code
$(eval $(call common.target,translations))

gettext.lint:
	@$(INFO) msgfmt check
	$(foreach p,$(PO_FILES),@$(MSGFMT) -c $(p) || $(FAIL) ${\n})
	@$(OK) msgfmt check

.gettext.merge:
	@$(INFO) msgmerge
	$(foreach l,$(LANGUAGES),@mkdir -p $(LOCALES_DIR)/$(l) || $(FAIL) ${\n})
	$(foreach pot,$(POT_FILES),$(foreach l,$(LANGUAGES), \
		@touch $(LOCALES_DIR)/$(l)/$(basename $(notdir $(pot))).po || $(FAIL) ${\n} \
		@$(MSGMERGE) -q --no-wrap --sort-output --no-fuzzy-matching --lang=$(l) -U "$(LOCALES_DIR)/$(l)/$(basename $(notdir $(pot))).po" "$(pot)" || $(FAIL) ${\n} \
	))
	@find $(LOCALES_DIR) -name '*.po~' -delete
	@find $(LOCALES_DIR) -name '*.pot~' -delete
	@$(OK) msgmerge

.gettext.build:
	@$(INFO) copying translations
	@rm -rf $(OUTPUT_DIR)/locales
	@cp -a $(LOCALES_DIR) $(OUTPUT_DIR)/locales
	@$(OK) copying translations

.PHONY: gettext.lint .gettext.build .gettext.merge

# ====================================================================================
# Common Targets
.lint.run: gettext.lint

.translations.run: .gettext.merge

.build.code: .gettext.build

endif # __GETTEXT_MAKEFILE__
