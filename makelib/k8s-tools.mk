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

ifndef __K8S_TOOLS_MAKEFILE__
__K8S_TOOLS_MAKEFILE__ := included

# ====================================================================================
# tools

# kubectl download and install
KUBECTL_VERSION ?= 1.19.13
KUBECTL_DOWNLOAD_URL ?= https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/$(HOSTOS)/$(HOSTARCH)/kubectl
$(eval $(call tool.download,kubectl,$(KUBECTL_VERSION),$(KUBECTL_DOWNLOAD_URL)))

# kind download and install
KIND_VERSION ?= 0.11.1
KIND_DOWNLOAD_URL ?= https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-$(HOSTOS)-$(HOSTARCH)
$(eval $(call tool.download,kind,$(KIND_VERSION),$(KIND_DOWNLOAD_URL)))

# kind download and install
KUSTOMIZE_VERSION ?= 4.2.0
KUSTOMIZE_DOWNLOAD_URL ?=https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_$(HOST_PLATFORM).tar.gz
$(eval $(call tool.download.tar.gz,kustomize,$(KUSTOMIZE_VERSION),$(KUSTOMIZE_DOWNLOAD_URL),kustomize,0))

endif # __K8S_TOOLS_MAKEFILE__

