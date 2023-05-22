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

ifndef __HELM_MAKEFILE__
__HELM_MAKEFILE__ := included

include $(COMMON_SELF_DIR)/k8s-tools.mk

# the charts directory
HELM_CHARTS_DIR ?= deploy/charts

HELM_CHARTS ?= $(patsubst $(HELM_CHARTS_DIR)/%,%,$(shell find $(HELM_CHARTS_DIR) -mindepth 1 -maxdepth 1 -type d))

# the base url where helm charts are published
# ifeq ($(HELM_BASE_URL),)
# $(error the variable HELM_BASE_URL must be set prior to including helm.mk)
# endif

# the charts output directory
HELM_OUTPUT_DIR ?= $(OUTPUT_DIR)/charts

# the helm index file
HELM_INDEX := $(HELM_OUTPUT_DIR)/index.yaml

# helm home
HELM_HOME := $(abspath $(WORK_DIR)/helm)
HELM_CHARTS_WORK_DIR := $(abspath $(WORK_DIR)/charts)
export HELM_HOME

# remove the leading `v` for helm chart versions
HELM_CHART_VERSION := $(VERSION:v%=%)
HELM_APP_VERSION ?= $(VERSION)

# ====================================================================================
# Tools install targets

HELM_VERSION := 3.6.3
HELM_DOWNLOAD_URL := https://get.helm.sh/helm-v$(HELM_VERSION)-$(HOSTOS)-$(HOSTARCH).tar.gz
$(eval $(call tool.download.tar.gz,helm,$(HELM_VERSION),$(HELM_DOWNLOAD_URL)))

# ====================================================================================
# Helm Targets

$(HELM_HOME): $(HELM)
	@mkdir -p $(HELM_HOME)

$(HELM_OUTPUT_DIR):
	@mkdir -p $(HELM_OUTPUT_DIR)

$(HELM_CHARTS_WORK_DIR):
	@mkdir -p $(HELM_CHARTS_WORK_DIR)

define helm.chart

.helm.package.init.$(1): $(HELM_CHARTS_WORK_DIR)
	@rm -rf $(HELM_CHARTS_WORK_DIR)/$(1)
	@cp -a $(abspath $(HELM_CHARTS_DIR)/$(1)) $(HELM_CHARTS_WORK_DIR)/$(1)
.helm.package.run.$(1): $(HELM_OUTPUT_DIR) $(HELM_HOME)
	@$(INFO) helm package $(1) $(HELM_CHART_VERSION)
	@$(HELM) package --version $(HELM_CHART_VERSION) --app-version $(HELM_APP_VERSION) -d $(HELM_OUTPUT_DIR) $(HELM_CHARTS_WORK_DIR)/$(1)
	@$(OK) helm package $(1) $(HELM_CHART_VERSION)
.helm.package.done.$(1): ; @:
.helm.package.$(1):
	@$(MAKE) .helm.package.init.$(1)
	@$(MAKE) .helm.package.run.$(1)
	@$(MAKE) .helm.package.done.$(1)

.PHONY: .helm.package.init.$(1) .helm.package.run.$(1) .helm.package.done.$(1) .helm.package.$(1)

$(HELM_OUTPUT_DIR)/$(1)-$(HELM_CHART_VERSION).tgz: $(HELM_HOME) $(HELM_OUTPUT_DIR) $(shell find $(HELM_CHARTS_DIR)/$(1) -type f)

.PHONY: .helm.lint.$(1)
.helm.lint.$(1): $(HELM_HOME)
	@$(INFO) helm lint $(1)
	@rm -rf $(abspath $(HELM_CHARTS_DIR)/$(1)/charts)
	@$(HELM) dependency build $(abspath $(HELM_CHARTS_DIR)/$(1))
	@$(HELM) lint $(abspath $(HELM_CHARTS_DIR)/$(1)) $(HELM_CHART_LINT_ARGS_$(1)) --strict || $$(FAIL)
	@$(OK) helm lint $(1)

helm.lint: .helm.lint.$(1)

.PHONY: .helm.dep.$(1)
.helm.dep.$(1): $(HELM_HOME)
	@$(INFO) helm dep $(1) $(HELM_CHART_VERSION)
	@$(HELM) dependency update $(abspath $(HELM_CHARTS_DIR)/$(1))
	@$(OK) helm dep $(1) $(HELM_CHART_VERSION)

helm.dep: .helm.dep.$(1)

$(HELM_INDEX): .helm.package.$(1)
endef
$(foreach p,$(HELM_CHARTS),$(eval $(call helm.chart,$(p))))

$(HELM_INDEX): $(HELM_HOME) $(HELM_OUTPUT_DIR)
	@$(INFO) helm index
	@$(HELM) repo index $(HELM_OUTPUT_DIR)
	@$(OK) helm index

helm.build: $(HELM_INDEX)

.helm.clean:
	@rm -fr $(HELM_OUTPUT_DIR)

.PHONY: helm.lint helm.build helm.dep .helm.clean

# ====================================================================================
# Common Targets

.build.check: helm.lint
.build.artifacts: helm.build
.lint.run: helm.lint
clean: .helm.clean

# ====================================================================================
# Special Targets

define HELM_HELPTEXT
Helm Targets:
    helm.dep           Upgrade charts dependencies

endef
export HELM_HELPTEXT

.PHONY: .helm.help
.helm.help:
	@echo "$$HELM_HELPTEXT"

.help: .helm.help

endif # __HELM_MAKEFILE__
