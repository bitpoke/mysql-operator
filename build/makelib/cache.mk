# Copyright 2019 Pressinfra SRL. All rights reserved.
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

ifndef __CACHE_MAKEFILE__
__CACHE_MAKEFILE__ := included

RCLONE_BIN ?= RCLONE_VERSION=true rclone
RCLONE_ARGS ?= -q --config /dev/null

ifeq ($(CACHE_BACKEND),)
$(error You must define CACHE_BACKEND before adding cache support. See format at https://rclone.org/docs/#backend-path-to-dir)
endif

CACHE_COMPRESSION ?= gzip

ifneq ($(DRONE_PULL_REQUEST),)
CACHE_NAME ?= $(PROJECT_NAME)-pr$(DRONE_PULL_REQUEST)-cache
else ifneq ($(DRONE_TAG),)
CACHE_NAME ?= $(PROJECT_NAME)-$(DRONE_TAG)-cache
else
CACHE_NAME ?= $(PROJECT_NAME)-$(BRANCH_NAME)-cache
endif


RCLONE := $(RCLONE_BIN) $(RCLONE_ARGS)

ifeq ($(CACHE_COMPRESSION),gzip)
TAR_COMPRESS_ARGS += -z
CACHE_EXTENSION_SUFFIX := .gz
endif

CACHE_FILE := $(CACHE_NAME).tar$(CACHE_EXTENSION_SUFFIX)

.PHONY: cache.store cache.restore

cache.store:
	@$(INFO) storing cache $(CACHE_FILE) into $(CACHE_BACKEND)
	@$(RCLONE) mkdir $(CACHE_BACKEND) || $(FAIL)
	@tar -C $(CACHE_DIR) $(TAR_COMPRESS_ARGS) -cf - ./ | $(RCLONE) rcat $(CACHE_BACKEND)/$(CACHE_FILE) || $(FAIL)
	@$(OK) cache store

cache.restore: |$(CACHE_DIR)
	@$(INFO) restoring cache from $(CACHE_BACKEND)/$(CACHE_FILE)
	@$(RCLONE) cat $(CACHE_BACKEND)/$(CACHE_FILE) | tar -C $(CACHE_DIR) $(TAR_COMPRESS_ARGS) -x \
		&& $(OK) cache restore \
		|| $(WARN) cache restore failed

endif # __CACHE_MAKEFILE__
