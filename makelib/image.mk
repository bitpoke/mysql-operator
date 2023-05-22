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

# ====================================================================================
# Options

ifndef __DOCKER_MAKEFILE__
__DOCKER_MAKEFILE__ := included

ifeq ($(origin IMAGE_DIR),undefined)
IMAGE_DIR := $(ROOT_DIR)/images
endif

ifeq ($(origin IMAGE_OUTPUT_DIR),undefined)
IMAGE_OUTPUT_DIR := $(OUTPUT_DIR)/images/$(PLATFORM)
endif

ifeq ($(origin IMAGE_TEMP_DIR),undefined)
IMAGE_TEMP_DIR := $(shell mktemp -d)
endif

ifeq ($(DRONE),true)
DOCKER_HOST ?= tcp://docker:2375
export DOCKER_HOST
endif

# a registry that is scoped to the current build tree on this host. this enables
# us to have isolation between concurrent builds on the same system, as in the case
# of multiple working directories or on a CI system with multiple executors. All images
# tagged with this build registry can safely be untagged/removed at the end of the build.
ifeq ($(origin BUILD_REGISTRY), undefined)
ifeq ($(CI_BUILD_NUMBER),)
BUILD_REGISTRY := build/$(shell echo $(HOSTNAME)-$(ROOT_DIR) | shasum -a 256 | cut -c1-8)
else
BUILD_REGISTRY := build/$(CI_BUILD_NUMBER)
endif
endif

# In order to reduce built time especially on jenkins, we maintain a cache
# of already built images. This cache contains images that can be used to help speed
# future docker build commands using docker's content addressable schemes.
# All cached images go in in a 'cache/' local registry and we follow an MRU caching
# policy -- keeping images that have been referenced around and evicting images
# that have to been referenced in a while (and according to a policy). Note we can
# not rely on the image's .CreatedAt date since docker only updates then when the
# image is created and not referenced. Instead we keep a date in the Tag.
CACHE_REGISTRY := cache

# prune images that are at least this many hours old
PRUNE_HOURS ?= 48

# prune keeps at least this many images regardless of how old they are
PRUNE_KEEP ?= 24

# don't actually prune just show what prune would do.
PRUNE_DRYRUN ?= 0

# the cached image format
CACHE_DATE_FORMAT := "%Y-%m-%d.%H%M%S"
CACHE_PRUNE_DATE := $(shell export TZ="UTC+$(PRUNE_HOURS)"; date +"$(CACHE_DATE_FORMAT)")
CACHE_TAG := $(shell date -u +"$(CACHE_DATE_FORMAT)")

REGISTRIES ?= $(DOCKER_REGISTRY)

# docker accepted image platform format
# eg linux/arm64 -> linux/arm64/v8, linux/armv7 -> linux/arm/v7, linux/armv6 -> linux/arm/v6
dockerify-platform = $(subst armv7,arm/v7,$(subst arm64,arm64/v8,$(1)))
IMAGE_ARCHS := $(subst linux_,,$(filter linux_%,$(PLATFORMS)))
IMAGE_PLATFORMS := $(call dockerify-platform,$(subst _,/,$(filter linux_%,$(PLATFORMS))))
IMAGE_PLATFORM = $(call dockerify-platform,linux/$(ARCH))

IMAGE_TAG ?= $(subst +,-,$(VERSION))

# if set to 1 docker image caching will not be used.
CACHEBUST ?= 0
ifeq ($(CACHEBUST),1)
BUILD_ARGS += --no-cache
endif

# if V=0 avoid showing verbose output from docker build
ifeq ($(V),0)
BUILD_ARGS ?= -q
endif

# if PULL=1 we will always check if there is a newer base image
PULL ?= 1
ifeq ($(PULL),1)
BUILD_BASE_ARGS += --pull
endif
BUILD_BASE_ARGS += $(BUILD_ARGS)
export PULL

ifeq ($(HOSTOS),Linux)
SELF_CID := $(shell cat /proc/self/cgroup | grep docker | grep -o -E '[0-9a-f]{64}' | head -n 1)
endif

# =====================================================================================
# Image Targets

.do.img.clean:
	@for i in $(CLEAN_IMAGES); do \
		if [ -n "$$(docker images -q $$i)" ]; then \
			for c in $$(docker ps -a -q --no-trunc --filter=ancestor=$$i); do \
				if [ "$$c" != "$(SELF_CID)" ]; then \
					$(INFO) stopping and removing container $${c} referencing image $$i; \
					docker stop $${c}; \
					docker rm $${c}; \
				fi; \
			done; \
			$(INFO) cleaning image $$i; \
			docker rmi $$i > /dev/null 2>&1 || true; \
		fi; \
	done

# this will clean everything for this build
.img.clean:
	@$(INFO) cleaning images for $(BUILD_REGISTRY)
	@$(MAKE) .do.img.clean CLEAN_IMAGES="$(shell docker images | grep -E '^$(BUILD_REGISTRY)/' | awk '{print $$1":"$$2}')"
	@$(OK) cleaning images for $(BUILD_REGISTRY)

.img.done:
	@rm -fr $(IMAGE_TEMP_DIR)

.img.cache:
	@for i in $(CACHE_IMAGES); do \
		IMGID=$$(docker images -q $$i); \
		if [ -n "$$IMGID" ]; then \
			$(INFO) caching image $$i; \
			CACHE_IMAGE=$(CACHE_REGISTRY)/$${i#*/}; \
			docker tag $$i $${CACHE_IMAGE}:$(CACHE_TAG); \
			for r in $$(docker images --format "{{.ID}}#{{.Repository}}:{{.Tag}}" | grep $$IMGID | grep $(CACHE_REGISTRY)/ | grep -v $${CACHE_IMAGE}:$(CACHE_TAG)); do \
				docker rmi $${r#*#} > /dev/null 2>&1 || true; \
			done; \
		fi; \
	done

# prune removes old cached images
img.prune:
	@$(INFO) pruning images older than $(PRUNE_HOURS) keeping a minimum of $(PRUNE_KEEP) images
	@EXPIRED=$$(docker images --format "{{.Tag}}#{{.Repository}}:{{.Tag}}" \
		| grep -E '$(CACHE_REGISTRY)/' \
		| sort -r \
		| awk -v i=0 -v cd="$(CACHE_PRUNE_DATE)" -F  "#" '{if ($$1 <= cd && i >= $(PRUNE_KEEP)) print $$2; i++ }') &&\
	for i in $$EXPIRED; do \
		$(INFO) removing expired cache image $$i; \
		[ $(PRUNE_DRYRUN) = 1 ] || docker rmi $$i > /dev/null 2>&1 || true; \
	done
	@for i in $$(docker images -q -f dangling=true); do \
		$(INFO) removing dangling image $$i; \
		docker rmi $$i > /dev/null 2>&1 || true; \
	done
	@$(OK) pruning

debug.nuke:
	@for c in $$(docker ps -a -q --no-trunc); do \
		if [ "$$c" != "$(SELF_CID)" ]; then \
			$(INFO) stopping and removing container $${c}; \
			docker stop $${c}; \
			docker rm $${c}; \
		fi; \
	done
	@for i in $$(docker images -q); do \
		$(INFO) removing image $$i; \
		docker rmi -f $$i > /dev/null 2>&1; \
	done

# 1: registry 2: image, 3: arch
define repo.targets
.PHONY: .img.release.build.$(1).$(2).$(3)
.img.release.build.$(1).$(2).$(3):
	@$(INFO) docker build $(1)/$(2):$(IMAGE_TAG)-$(3)
	@docker tag $(BUILD_REGISTRY)/$(2)-$(3) $(1)/$(2):$(IMAGE_TAG)-$(3) || $(FAIL)
	@$(OK) docker build $(1)/$(2):$(IMAGE_TAG)-$(3)
.img.release.build: .img.release.build.$(1).$(2).$(3)

.PHONY: .img.release.publish.$(1).$(2).$(3)
.img.release.publish.$(1).$(2).$(3):
	@$(INFO) docker push $(1)/$(2):$(IMAGE_TAG)-$(3)
	@docker push $(1)/$(2):$(IMAGE_TAG)-$(3) || $(FAIL)
	@$(OK) docker push $(1)/$(2):$(IMAGE_TAG)-$(3)
.img.release.publish: .img.release.publish.$(1).$(2).$(3)

.PHONY: .img.release.promote.$(1).$(2).$(3)
.img.release.promote.$(1).$(2).$(3):
	@$(INFO) docker promote $(1)/$(2):$(IMAGE_TAG)-$(3) to $(1)/$(2)-$(3):$(CHANNEL)
	@docker pull $(1)/$(2):$(IMAGE_TAG)-$(3) || $(FAIL)
	@[ "$(CHANNEL)" = "master" ] || docker tag $(1)/$(2):$(IMAGE_TAG)-$(3) $(1)/$(2):$(IMAGE_TAG)-$(3)-$(CHANNEL) || $(FAIL)
	@docker tag $(1)/$(2):$(IMAGE_TAG)-$(3) $(1)/$(2)-$(3):$(CHANNEL) || $(FAIL)
	@[ "$(CHANNEL)" = "master" ] || docker push $(1)/$(2):$(IMAGE_TAG)-$(3)-$(CHANNEL)
	@docker push $(1)/$(2)-$(3):$(CHANNEL) || $(FAIL)
	@$(OK) docker promote $(1)/$(2):$(IMAGE_TAG)-$(3) to $(1)/$(2)-$(3):$(CHANNEL) || $(FAIL)
.img.release.promote: .img.release.promote.$(1).$(2).$(3)

.PHONY: .img.release.clean.$(1).$(2).$(3)
.img.release.clean.$(1).$(2).$(3):
	@[ -z "$$$$(docker images -q $(1)/$(2):$(IMAGE_TAG)-$(3))" ] || docker rmi $(1)/$(2):$(IMAGE_TAG)-$(3)
	@[ -z "$$$$(docker images -q $(1)/$(2):$(IMAGE_TAG)-$(3)-$(CHANNEL))" ] || docker rmi $(1)/$(2):$(IMAGE_TAG)-$(3)-$(CHANNEL)
	@[ -z "$$$$(docker images -q $(1)/$(2)-$(3):$(CHANNEL))" ] || docker rmi $(1)/$(2)-$(3):$(CHANNEL)
.img.release.clean: .img.release.clean.$(1).$(2).$(3)
endef
$(foreach r,$(REGISTRIES), $(foreach i,$(IMAGES), $(foreach a,$(IMAGE_ARCHS),$(eval $(call repo.targets,$(r),$(i),$(a))))))

.PHONY: .img.release.manifest.publish.%
.img.release.manifest.publish.%: .img.release.publish
	@docker buildx imagetools create --tag $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG) $(patsubst %,$(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)-%,$(IMAGE_ARCHS))

.PHONY: .img.release.manifest.promote.%
.img.release.manifest.promote.%: .img.release.promote
	@[ "$(CHANNEL)" = "master" ] || docker buildx imagetools create --tag $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)-$(CHANNEL) $(patsubst %,$(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)-%,$(IMAGE_ARCHS)) || $(FAIL)
	@docker buildx imagetools create --tag $(DOCKER_REGISTRY)/$*:$(CHANNEL) $(patsubst %,$(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)-%,$(IMAGE_ARCHS))

.img.release.build: ;@
.img.release.publish: ;@
.img.release.promote: ;@
.img.release.clean: ;@

.PHONY: img.prune .img.done .img.clean .do.img.clean .img.release.build .img.release.publish .img.release.promote
.PHONY: .img.release.clean .img.cache img.publish

# ====================================================================================
# Common Targets

# if IMAGES is defined then invoke and build each image identified
ifneq ($(IMAGES),)

ifeq ($(DOCKER_REGISTRY),)
$(error the variable DOCKER_REGISTRY must be set prior to including image.mk)
endif

.do.build.image.%:
ifeq ($(filter linux_%,$(PLATFORM)),)
	@$(WARN) skipping docker build for $* on PLATFORM=$(PLATFORM)
else
	@$(MAKE) -C $(IMAGE_DIR)/$* PLATFORM=$(PLATFORM)
endif

.do.build.images: $(foreach i,$(IMAGES), .do.build.image.$(i)) ;
.build.artifacts.platform: .do.build.images
.build.done: .img.cache .img.done
clean: .img.clean .img.release.clean

.publish.init: .img.release.build

img.publish: $(addprefix .img.release.manifest.publish.,$(IMAGES))

# only publish images for master and release branches
ifneq ($(filter master release-%,$(BRANCH_NAME)),)
.publish.run: img.publish
endif

.promote.run: $(addprefix .img.release.manifest.promote.,$(IMAGES))

else # assume this .mk file is being included to build a single image

ifeq ($(PLATFORM),darwin_amd64) # when building docker image on macOS pretend we are building for linux
PLATFORM := linux_amd64
endif

ifneq ($(filter $(PLATFORM),$(PLATFORMS)),)
.build.artifacts.platform: img.build
.build.done: .img.cache .img.done
clean: .img.clean
else # trying to build a docker image for an invalid platform
.DEFAULT_GOAL := .skip
.PHONY: .skip
.skip:
	@$(WARN) skipping docker build for $(IMAGE) for PLATFORM=$(PLATFORM)
endif

endif

# ====================================================================================
# Special Targets

define IMAGE_HELPTEXT
Image Targets:
    img.prune          Prune orphaned and cached images.

Image Options:
    PRUNE_HOURS        The number of hours from when an image is last used
                       for it to be considered a target for pruning.
                       Default is 48 hours.
    PRUNE_KEEP         The minimum number of cached images to keep.
                       Default is 24 images.

endef
export IMAGE_HELPTEXT

.img.help:
	@echo "$$IMAGE_HELPTEXT"

.help: .img.help

.PHONY: .img.help

endif # __DOCKER_MAKEFILE__
