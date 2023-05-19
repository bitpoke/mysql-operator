# Copyright 2019 Pressinfra Authors. All rights reserved.
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

ifndef __KUBEBUILDERV3_MAKEFILE__
__KUBEBUILDERV3_MAKEFILE__ := included

include $(COMMON_SELF_DIR)/golang.mk

# ====================================================================================
# Options

CRD_DIR ?= config/crd/bases
RBAC_DIR ?= config/rbac

BOILERPLATE_FILE ?= hack/boilerplate.go.txt

CONTROLLER_GEN_CRD_OPTIONS ?= crd output:crd:artifacts:config=$(CRD_DIR)
CONTROLLER_GEN_RBAC_OPTIONS ?= rbac:roleName=manager-role output:rbac:artifacts:config=$(RBAC_DIR)
CONTROLLER_GEN_WEBHOOK_OPTIONS ?= webhook
CONTROLLER_GEN_OBJECT_OPTIONS ?= object:headerFile=$(BOILERPLATE_FILE)
CONTROLLER_GEN_PATHS ?= $(foreach t,$(GO_SUBDIRS),paths=./$(t)/...)

KUBEBUILDER_ASSETS_VERSION ?= 1.19.2
KUBEBUILDER_ASSETS = $(CACHE_DIR)/kubebuilder/k8s/$(KUBEBUILDER_ASSETS_VERSION)-$(HOSTOS)-$(HOSTARCH)
export KUBEBUILDER_ASSETS

# ====================================================================================
# tools

# setup-envtest download and install
SETUP_ENVTEST_VERSION ?= 0.0.0-20211206022232-3ffc700bc2a3
SETUP_ENVTEST_DOWNLOAD_URL ?= sigs.k8s.io/controller-runtime/tools/setup-envtest
$(eval $(call tool.go.install,setup-envtest,v$(SETUP_ENVTEST_VERSION),$(SETUP_ENVTEST_DOWNLOAD_URL)))

# kubebuilder download and install
KUBEBUILDER_VERSION ?= 3.2.0
KUBEBUILDER_DOWNLOAD_URL ?= https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(HOST_PLATFORM)
$(eval $(call tool.download,kubebuilder,$(KUBEBUILDER_VERSION),$(KUBEBUILDER_DOWNLOAD_URL)))

# controller-gen download and install
CONTROLLER_GEN_VERSION ?= 0.7.0
CONTROLLER_GEN_DOWNLOAD_URL ?= sigs.k8s.io/controller-tools/cmd/controller-gen
$(eval $(call tool.go.install,controller-gen,v$(CONTROLLER_GEN_VERSION),$(CONTROLLER_GEN_DOWNLOAD_URL)))

build.tools: |$(KUBEBUILDER_ASSETS)
$(KUBEBUILDER_ASSETS): $(SETUP_ENVTEST)
	@echo ${TIME} ${BLUE}[TOOL]${CNone} installing kubebuilder assets for Kubernetes $(KUBEBUILDER_ASSETS_VERSION)
	@$(SETUP_ENVTEST) --bin-dir=$(CACHE_DIR)/kubebuilder --os=$(HOSTOS) --arch=$(HOSTARCH) use $(KUBEBUILDER_ASSETS_VERSION)
	@$(OK) installing kubebuilder assets for Kubernetes $(KUBEBUILDER_ASSETS_VERSION)

# ====================================================================================
# Kubebuilder Targets

$(eval $(call common.target,kubebuilder.manifests))
# Generate manifests e.g. CRD, RBAC etc.
.do.kubebuilder.manifests: $(CONTROLLER_GEN)
	@$(INFO) Generating Kubernetes \(CRDs, RBAC, WebhookConfig, etc.\) manifests
	@$(CONTROLLER_GEN) \
		$(CONTROLLER_GEN_CRD_OPTIONS) \
		$(CONTROLLER_GEN_RBAC_OPTIONS) \
		$(CONTROLLER_CONTROLLER_GEN_WEBHOOK_OPTIONS) \
		$(CONTROLLER_GEN_PATHS)
	@$(OK) Generating Kubernetes \(CRDs, RBAC, WebhookConfig, etc.\) manifests
.PHONY: .do.kubebuilder.manifests
.kubebuilder.manifests.run: .do.kubebuilder.manifests

$(eval $(call common.target,kubebuilder.code))
# Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
.do.kubebuilder.code: $(CONTROLLER_GEN)
	@$(INFO) Generating DeepCopy, DeepCopyInto, and DeepCopyObject code
	@$(CONTROLLER_GEN) \
		$(CONTROLLER_GEN_OBJECT_OPTIONS) \
		$(CONTROLLER_GEN_PATHS)
	@$(OK) Generating DeepCopy, DeepCopyInto, and DeepCopyObject code
.PHONY: .do.kubebuilder.code
.kubebuilder.code.run: .do.kubebuilder.code

# ====================================================================================
# Common Targets

.test.init: |$(KUBEBUILDER_ASSETS)
go.test.unit: |$(KUBEBUILDER_ASSETS)
go.generate: kubebuilder.code
.generate.init: .kubebuilder.manifests.init
.generate.run: .kubebuilder.manifests.run
.generate.done: .kubebuilder.manifests.done

# ====================================================================================
# Special Targets

define KUBEBULDERV3_HELPTEXT
Kubebuilder Targets:
    kubebuilder.manifests   Generates Kubernetes custom resources manifests (e.g. CRDs RBACs, ...)
    kubebuilder.code        Generates DeepCopy, DeepCopyInto, and DeepCopyObject code

endef
export KUBEBULDERV3_HELPTEXT

.kubebuilder.help:
	@echo "$$KUBEBULDERV3_HELPTEXT"

.help: .kubebuilder.help
.PHONY: .kubebuilder.help

endif # __KUBEBUILDERV3_MAKEFILE__
