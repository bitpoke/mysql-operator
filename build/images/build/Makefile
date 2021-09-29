PLATFORMS := linux_amd64
include ../../makelib/common.mk

# this is required, since by default, the makelib files are under a ./build path prefix, but here,
# they are under root
ROOT_DIR := $(abspath $(shell cd ./../.. && pwd -P))

IMAGE = $(BUILD_REGISTRY)/build-$(ARCH)
CACHE_IMAGES = $(IMAGE)
include ../../makelib/image.mk

img.build:
	@$(INFO) docker build $(IMAGE) $(IMAGE_PLATFORM)
	@cp -La . $(IMAGE_TEMP_DIR)
	@mkdir -p $(IMAGE_TEMP_DIR)/rootfs
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORM) \
		-t $(IMAGE) \
		--build-arg ARCH=$(ARCH) \
		$(IMAGE_TEMP_DIR)
	@$(OK) docker build $(IMAGE)
