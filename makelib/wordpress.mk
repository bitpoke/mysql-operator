# Copyright 2020 The Pressinfra Authors. All rights reserved.
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

ifndef __WORDPRESS_MAKEFILE__
__WORDPRESS_MAKEFILE__ := included

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(SELF_DIR)/php.mk

# ====================================================================================
# Options

WP_VERSION ?= master

WP_OUTPUT_DIR := $(OUTPUT_DIR)/wordpress-$(WP_VERSION)

ifeq ($(WP_VERSION),master)
WP_DOWNLOAD_URL ?= https://github.com/WordPress/wordpress-develop/archive/$(WP_VERSION).tar.gz
else
WP_DOWNLOAD_URL ?= https://wordpress.org/wordpress-$(WP_VERSION).tar.gz
endif
WP_ARCHIVE := $(CACHE_DIR)/wordpress-$(WP_VERSION).tar.gz


WP_TESTS_VERSION ?= $(WP_VERSION)
WP_TESTS_DIR ?= $(WORK_DIR)/wordpress-develop-$(WP_VERSION)
WP_TESTS_CONFIG ?= $(abspath $(PHP_ROOT_DIR))/tests/wp-tests-config.php
WP_TESTS_DOWNLOAD_URL ?= https://github.com/WordPress/wordpress-develop/archive/$(WP_VERSION).tar.gz
WP_TESTS_ARCHIVE := $(CACHE_DIR)/wordpress-develop-$(WP_VERSION).tar.gz

# =====================================================================================
# WordPress Targets

$(WP_TESTS_ARCHIVE):
	@$(INFO) fetching $(notdir $@) from $(WP_TESTS_DOWNLOAD_URL)
	@curl -sLo "$@" "$(WP_TESTS_DOWNLOAD_URL)" || $(FAIL)
	@$(OK) fetching $(notdir $@) from $(WP_TESTS_DOWNLOAD_URL)

$(WP_TESTS_DIR)/src/wp-includes/version.php: $(WP_TESTS_ARCHIVE)
	@$(INFO) unpacking $<
	@rm -rf $(WP_TESTS_DIR) && mkdir -p $(WP_TESTS_DIR)
	@tar -zxf $< -C $(WP_TESTS_DIR) --strip-components 1
	@cp tests/wp-tests-config.php $(WP_TESTS_DIR)
	@mkdir -p $(WP_TESTS_DIR)/src/wp-content/uploads
	@test -f $@ && touch $@ || $(FAIL)
	@$(OK) unpacking $<

$(WP_TESTS_DIR)/wp-tests-config.php: $(WP_TESTS_CONFIG) $(WP_TESTS_DIR)/src/wp-includes/version.php
	@cp $(WP_TESTS_CONFIG) $@

# add WP_TESTS_DIR env var for running tests
.do.php.test: PHPUNIT:=WP_TESTS_DIR=$(WP_TESTS_DIR) $(PHPUNIT)

$(WP_ARCHIVE):
	@$(INFO) fetching $(notdir $@) from $(WP_DOWNLOAD_URL)
	@curl -sLo "$@" "$(WP_DOWNLOAD_URL)" || $(FAIL)
	@$(OK) fetching $(notdir $@) from $(WP_DOWNLOAD_URL)

$(WP_OUTPUT_DIR)/wp-includes/version.php: $(WP_ARCHIVE)
	@$(INFO) unpacking $<
	@rm -rf $(WP_OUTPUT_DIR) && mkdir -p $(WP_OUTPUT_DIR)
	@tar -zxf $< -C $(WP_OUTPUT_DIR) --strip-components 1
	@test -f $@ && touch $@ || $(FAIL)
	@$(OK) unpacking $<

$(eval $(call common.target,wordpress.build))
.wordpress.build.init: $(WP_OUTPUT_DIR)/wp-includes/version.php

# ====================================================================================
# Common Targets

.php.test.init: $(WP_TESTS_DIR)/wp-tests-config.php
.build.artifacts: wordpress.build

endif # __WORDPRESS_MAKEFILE__
