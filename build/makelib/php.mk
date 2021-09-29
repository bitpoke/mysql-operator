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

ifndef __PHP_MAKEFILE__
__PHP_MAKEFILE__ := included

# ====================================================================================
# Options

# supported php versions
PHP_SUPPORTED_VERSIONS ?= 7.3|7.4
PHP := php

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

# The location of php application within this git repo.
PHP_ROOT_DIR ?= $(SELF_DIR)/../..

# The location of php application source code, relative to the PHP_ROOT_DIR
PHP_SRC_DIR ?= src

COMPOSER_VERSION ?= 1.10.5
COMPOSER_DOWNLOAD_URL ?= https://getcomposer.org/download/$(COMPOSER_VERSION)/composer.phar
$(eval $(call tool.download,composer,$(COMPOSER_VERSION),$(COMPOSER_DOWNLOAD_URL)))

COMPOSER_INSTALL_ARGS ?= --prefer-dist --classmap-authoritative
COMPOSER_VENDOR_DIR := $(PHP_ROOT_DIR)/vendor
COMPOSER_BIN_DIR := $(abspath $(PHP_ROOT_DIR)/vendor/bin)
COMPOSER_JSON_FILE := $(PHP_ROOT_DIR)/composer.json
COMPOSER_LOCK_FILE := $(PHP_ROOT_DIR)/composer.lock

COMPOSER_CACHE_DIR := $(CACHE_DIR)/composer
export COMPOSER_CACHE_DIR

# ====================================================================================
# PHP Tools Targets

PHPUNIT := $(COMPOSER_BIN_DIR)/phpunit
$(PHPUNIT): |composer.install
build.tools: $(PHPUNIT)

PHPCS := $(COMPOSER_BIN_DIR)/phpcs
$(PHPCS): |composer.install
build.tools: $(PHPCS)

PHPCBF := $(COMPOSER_BIN_DIR)/phpcbf
$(PHPCBF): |composer.install
build.tools: $(PHPCBF)

# ====================================================================================
# Composer targets

.PHONY: .composer.init
.composer.init:
	@if ! `$(PHP) --version | grep -q -E '^PHP ($(PHP_SUPPORTED_VERSIONS))\.'`; then \
		$(ERR) unsupported PHP version. Please install one of the following supported version: '$(PHP_SUPPORTED_VERSIONS)' ;\
		exit 1 ;\
	fi
$(COMPOSER): .composer.init

COMPOSER_INSTALL_STAMP := $(COMPOSER_VENDOR_DIR)/.composer.install.stamp

# only run "composer" if the composer.json has changed
$(COMPOSER_INSTALL_STAMP): $(COMPOSER) $(COMPOSER_JSON_FILE) $(COMPOSER_LOCK_FILE)
	@echo ${TIME} $(BLUE)[TOOL]$(CNone) composer install
	@cd $(PHP_ROOT_DIR); $(COMPOSER) install --no-interaction || $(FAIL)
	@touch $(COMPOSER_INSTALL_STAMP)
	@$(OK) composer install

composer.install: $(COMPOSER_INSTALL_STAMP)

composer.update: $(COMPOSER)
	@echo ${TIME} $(BLUE)[TOOL]$(CNone) composer update
	@cd $(PHP_ROOT_DIR); $(COMPOSER) update || $(FAIL)
	@touch $(COMPOSER_INSTALL_STAMP)
	@$(OK) composer install


.composer.clean:
	@rm -rf $(COMPOSER_VENDOR_DIR)

.PHONY: composer.install composer.update .composer.clean

# ====================================================================================
# PHP Targets

php.lint:
	@$(INFO) phpcs $(PHP_SRC_DIR)
	@cd $(PHP_ROOT_DIR); $(PHPCS) $(PHP_SRC_DIR)
	@$(OK) phpcs $(PHP_SRC_DIR)

php.lint.fix:
	@$(INFO) phpcbf $(PHP_SRC_DIR)
	@cd $(PHP_ROOT_DIR); $(PHPCBF) $(PHP_SRC_DIR)
	@$(OK) phpcbf $(PHP_SRC_DIR)
.PHONY: php.lint php.lint.fix

# common target for building a php project
$(eval $(call common.target,php.build))

# common target for testing a php project
$(eval $(call common.target,php.test))

.PHONY: .do.php.test
.php.test.run: .do.php.test
.do.php.test: $(PHPUNIT)
	@$(INFO) phpunit
	@$(PHPUNIT) $(PHPUNIT_ARGS)
	@$(OK) phpunit


# ====================================================================================
# Common Targets

.build.init: .composer.init
.build.check: php.lint
.build.code: .php.build.run
.build.done: .php.build.done

.test.init: .php.test.init
.test.run: .php.test.run
.test.done: .php.test.done

clean: .composer.clean

.lint.run: php.lint
.fmt.run: php.lint.fix

# ====================================================================================
# Special Targets

define PHP_HELPTEXT
PHP Targets:
    composer.install       Installs dependencies in a make friendly manner.
    composer.update        Updates dependencies in a make friendly manner.

endef
export PHP_HELPTEXT

.PHONY: .php.help
.php.help:
	@echo "$$PHP_HELPTEXT"

.help: .php.help

endif # __PHP_MAKEFILE__
