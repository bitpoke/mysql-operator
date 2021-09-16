PLATFORMS := linux_amd64
include ../../build/makelib/common.mk

IMAGE ?= $(BUILD_REGISTRY)/mysql-operator-sidecar-5.7-$(ARCH)
CACHE_IMAGES = $(IMAGE)
include ../../build/makelib/image.mk

img.build:
	@$(INFO) docker build $(IMAGE) $(IMAGE_PLATFORM)
	@cp -La . $(IMAGE_TEMP_DIR)
	@cp $(OUTPUT_DIR)/bin/linux_$(ARCH)/mysql-operator-sidecar $(IMAGE_TEMP_DIR)/rootfs/usr/local/bin/mysql-operator-sidecar
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORM) \
		-t $(IMAGE) \
		--build-arg ARCH=$(ARCH) \
		$(IMAGE_TEMP_DIR)
	@$(OK) docker build $(IMAGE)
