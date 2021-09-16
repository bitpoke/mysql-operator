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

ifndef __GIT_PUBLISH_MAKEFILE__
__GIT_PUBLISH_MAKEFILE__ := included

# ====================================================================================
# Options

PUBLISH_BRANCH ?= master
PUBLISH_PREFIX ?= /
PUBLISH_TAGS ?= true

ifeq ($(PUBLISH_DIRS),)
PUBLISH_DIR ?= $(CURDIR:$(abspath $(ROOT_DIR))/%=%)
endif

PUBLISH_WORK_BRANCH := build/split-$(COMMIT_HASH)/$(PUBLISH_DIR)
PUBLISH_WORK_DIR := $(WORK_DIR)/git-publish/$(PUBLISH_DIR)
PUBLISH_GIT := git -C $(PUBLISH_WORK_DIR)

GIT_MERGE_ARGS ?= --ff-only
GIT_SUBTREE_MERGE_ARGS ?= --squash
ifeq ($(PUBLISH_TAGS),true)
GIT_PUSH_ARGS := --follow-tags
endif

PUBLISH_DIRS ?= $(PUBLISH_DIR)

# ====================================================================================
# git publish targets

git.urlize = $(patsubst %,https://%,$(patsubst %.git,%,$(patsubst https://%,%,$(patsubst git@github.com:%,https://github.com/%,$(1)))))
git.workbranch = build/split-$(COMMIT_HASH)/$(1)

# 1 publish directory
define git.publish

$(ROOT_DIR)/.git/refs/heads/$(call git.workbranch,$(1)):
	@$(INFO) git subtree split $(1)
	@cd $(ROOT_DIR) && git subtree split -q -P $(1) -b $(call git.workbranch,$(1)) $(COMMIT_HASH)
	@$(OK) git subtree split $(1)
.PHONY: .git.build.artifacts.$(1)
.git.build.artifacts.$(1): $(ROOT_DIR)/.git/refs/heads/$(call git.workbranch,$(1))
.git.build.artifacts: .git.build.artifacts.$(1)

.PHONY: .git.clean.$(1)
.git.clean.$(1):
	@cd $(ROOT_DIR) && git branch -D $(call git.workbranch,$(1)) || true
.git.clean: .git.clean.$(1)

.PHONY: .do.git.publish.$(1)
.do.git.publish.$(1): |$(ROOT_DIR)/.git/refs/heads/$(call git.workbranch,$(1))
	@$(MAKE) -C $(1) .git.publish

endef

ifeq ($(filter-out $(PUBLISH_DIR),$(PUBLISH_DIRS)),)
.git.publish: |$(ROOT_DIR)/.git/refs/heads/$(PUBLISH_WORK_BRANCH)
	@$(INFO) Publishing $(1) to $(PUBLISH_REPO)@$(PUBLISH_BRANCH) under $(PUBLISH_PREFIX)
	@rm -rf $(PUBLISH_WORK_DIR) && mkdir -p $(PUBLISH_WORK_DIR)
	@$(PUBLISH_GIT) init -q
	@$(PUBLISH_GIT) remote add origin $(PUBLISH_REPO)
	@$(PUBLISH_GIT) remote add upstream $(ROOT_DIR)/.git
	@$(PUBLISH_GIT) fetch -q upstream +refs/heads/$(PUBLISH_WORK_BRANCH):
	@$(PUBLISH_GIT) checkout -q -b $(PUBLISH_BRANCH)
	@set -e; cd $(PUBLISH_WORK_DIR); if git ls-remote --heads origin | grep -q refs/heads/$(PUBLISH_BRANCH); then \
		$(PUBLISH_GIT) fetch -q origin +refs/heads/$(PUBLISH_BRANCH): ;\
		$(PUBLISH_GIT) reset -q --hard origin/$(PUBLISH_BRANCH) ;\
		$(PUBLISH_GIT) branch -q -u origin/$(PUBLISH_BRANCH) ;\
	fi
ifeq ($(PUBLISH_PREFIX),/)
	@set -e; \
	$(PUBLISH_GIT) merge -q $(GIT_MERGE_ARGS) \
		-m "Merge '$(PUBLISH_DIR)' from $(patsubst https://github.com/%,%,$(call git.urlize,$(REMOTE_URL)))@$(COMMIT_HASH)" \
		upstream/$(PUBLISH_WORK_BRANCH) ;\
	if [ "$(PUBLISH_TAGS)" == "true" ] ; then \
		for t in $(TAGS) ; do \
			$(PUBLISH_GIT) tag -a -m "$$t" $$t ;\
		done ;\
	fi
else
	@set -e; \
	if [ -d "$(PUBLISH_WORK_DIR)/$(PUBLISH_PREFIX)" ] ; then \
		$(PUBLISH_GIT) subtree -q merge -P $(PUBLISH_PREFIX) $(GIT_SUBTREE_MERGE_ARGS) \
		-m "Merge '$(PUBLISH_DIR)' from $(patsubst https://github.com/%,%,$(call git.urlize,$(REMOTE_URL)))@$(COMMIT_HASH)" \
		upstream/$(PUBLISH_WORK_BRANCH) ;\
	else \
		$(PUBLISH_GIT) subtree add -q -P $(PUBLISH_PREFIX) $(GIT_SUBTREE_MERGE_ARGS) \
		-m "Add '$(PUBLISH_DIR)' from $(patsubst https://github.com/%,%,$(call git.urlize,$(REMOTE_URL)))@$(COMMIT_HASH)" \
			$(ROOT_DIR)/.git $(PUBLISH_WORK_BRANCH) ;\
	fi
endif
	@$(PUBLISH_GIT) push -u origin $(GIT_PUSH_ARGS) $(PUBLISH_BRANCH)
	@$(OK) Published $(1) to $(PUBLISH_REPO)@$(PUBLISH_BRANCH)
else
.git.publish: $(foreach d,$(PUBLISH_DIRS),.do.git.publish.$(d))
endif

$(foreach d,$(PUBLISH_DIRS), $(eval $(call git.publish,$(d))))

.PHONY: .git.clean .git.build.artifacts .git.publish

# ====================================================================================
# Common Targets

# if PUBLISH_DIRS is defined the invoke publish for each dir
ifneq ($(filter-out $(PUBLISH_DIR),$(PUBLISH_DIRS)),)

.publish.init: .git.build.artifacts
clean: .git.clean

# only publish for master and release branches
# also, if publishing for tags is enabled,
# publish if the current commit is a tag
ifneq ($(filter master release-%,$(BRANCH_NAME)),)
.publish.run: $(addprefix .do.git.publish.,$(PUBLISH_DIRS))
else ifeq ($(PUBLISH_TAGS),true)
ifneq ($(TAGS),)
.publish.run: $(addprefix .do.git.publish.,$(PUBLISH_DIRS))
endif
endif

else # assume this .mk file is being included for a single dir

ifeq ($(PUBLISH_REPO),)
$(error You must specify the PUBLISH_REPO variable in order to handle git publishing)
endif

.publish.init: .git.build.artifacts
clean: .git.clean

endif # PUBLISH_DIRS


endif # __GIT_PUBLISH_MAKEFILE__
